package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type policyTransport func(*http.Request) (*http.Response, error)

func (f policyTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type probeBody struct{ reads, closes int }

func (b *probeBody) Read([]byte) (int, error) { b.reads++; return 0, io.EOF }
func (b *probeBody) Close() error             { b.closes++; return nil }

var _ = Describe("AppImage URL policy contracts", func() {
	const source = "https://example.test/tool.AppImage"
	const target = "https://cdn.example.test/tool.AppImage"
	var plugin *AppimagePlugin
	var calls []string
	var respond func(*http.Request) (*http.Response, error)
	response := func(req *http.Request, status int, location string) *http.Response {
		h := make(http.Header)
		if location != "" {
			h.Set("Location", location)
		}
		return &http.Response{StatusCode: status, Status: fmt.Sprintf("%d fixture", status), Header: h, Body: io.NopCloser(strings.NewReader("not an AppImage")), Request: req}
	}
	BeforeEach(func() {
		sdktest.NewRecorder(GinkgoT(), "AppImage")
		plugin = NewAppimagePlugin()
		previous := http.DefaultTransport
		http.DefaultTransport = policyTransport(func(r *http.Request) (*http.Response, error) {
			calls = append(calls, r.Method+" "+r.URL.String())
			return respond(r)
		})
		DeferCleanup(func() { http.DefaultTransport = previous })
		calls = nil
		respond = func(r *http.Request) (*http.Response, error) { return response(r, 200, ""), nil }
	})
	for _, command := range []string{"validate-url", "install"} {
		Context(command, func() {
			args := func(url string) []string {
				if command == "install" {
					return []string{url, "tool", "--cli"}
				}
				return []string{url}
			}
			expectedSuccess := func(probes ...string) []string {
				if command == "install" {
					return append(probes, "GET "+source)
				}
				return probes
			}
			for _, url := range []string{"http://example.test/tool", "file:///tmp/tool", "https:///tool", "https://localhost/tool", "https://192.168.1.1/tool", ":bad"} {
				It("rejects "+url+" before transport", func() {
					Expect(plugin.Execute(command, args(url))).To(HaveOccurred())
					Expect(calls).To(BeEmpty())
				})
			}
			It("accepts a 2xx probe without claiming to verify content", func() {
				Expect(plugin.Execute(command, args(source))).To(Succeed())
				Expect(calls).To(Equal(expectedSuccess("HEAD " + source)))
			})
			for _, status := range []int{405, 501} {
				for _, fallbackStatus := range []int{200, 206} {
					It(fmt.Sprintf("falls back from HEAD %d to ranged GET %d", status, fallbackStatus), func() {
						body := &probeBody{}
						respond = func(r *http.Request) (*http.Response, error) {
							if r.Method == http.MethodHead {
								return response(r, status, ""), nil
							}
							if r.Header.Get("Range") != "" {
								Expect(r.Header.Get("Range")).To(Equal("bytes=0-0"))
								resp := response(r, fallbackStatus, "")
								resp.Body = body
								return resp, nil
							}
							return response(r, 200, ""), nil
						}
						Expect(plugin.Execute(command, args(source))).To(Succeed())
						Expect(calls).To(Equal(expectedSuccess("HEAD "+source, "GET "+source)))
						Expect(body.reads).To(BeZero())
						Expect(body.closes).To(Equal(1))
					})
				}
			}
			for _, status := range []int{300, 304, 403, 404, 500} {
				It(fmt.Sprintf("rejects HEAD %d without fallback", status), func() {
					respond = func(r *http.Request) (*http.Response, error) { return response(r, status, ""), nil }
					Expect(plugin.Execute(command, args(source))).To(MatchError(ContainSubstring(fmt.Sprintf("URL returned status %d", status))))
					Expect(calls).To(Equal([]string{"HEAD " + source}))
				})
			}
			It("returns failed fallback status and does not install", func() {
				respond = func(r *http.Request) (*http.Response, error) {
					if r.Method == http.MethodHead {
						return response(r, 405, ""), nil
					}
					return response(r, 403, ""), nil
				}
				Expect(plugin.Execute(command, args(source))).To(MatchError(ContainSubstring("URL returned status 403")))
				Expect(calls).To(Equal([]string{"HEAD " + source, "GET " + source}))
				Expect(filepath.Join(os.Getenv("HOME"), ".local/bin/tool")).NotTo(BeAnExistingFile())
			})
			It("follows HTTPS redirects for probes and downloads", func() {
				respond = func(r *http.Request) (*http.Response, error) {
					if r.URL.String() == source {
						return response(r, 302, target), nil
					}
					return response(r, 200, ""), nil
				}
				Expect(plugin.Execute(command, args(source))).To(Succeed())
				expected := []string{"HEAD " + source, "HEAD " + target}
				if command == "install" {
					expected = append(expected, "GET "+source, "GET "+target)
				}
				Expect(calls).To(Equal(expected))
			})
			for _, location := range []string{"http://cdn.example.test/tool", "https://localhost/tool", "https://10.0.0.1/tool"} {
				It("rejects redirect to "+location+" before transport", func() {
					respond = func(r *http.Request) (*http.Response, error) { return response(r, 302, location), nil }
					Expect(plugin.Execute(command, args(source))).To(HaveOccurred())
					Expect(calls).To(Equal([]string{"HEAD " + source}))
				})
			}
			It("uses the final HEAD URL for fallback and follows its relative redirect", func() {
				respond = func(r *http.Request) (*http.Response, error) {
					switch {
					case r.URL.String() == source && r.Method == http.MethodHead:
						return response(r, 302, target), nil
					case r.URL.String() == target && r.Method == http.MethodHead:
						return response(r, 405, ""), nil
					case r.URL.String() == target && r.Header.Get("Range") != "":
						return response(r, 302, "/probe"), nil
					case r.URL.Path == "/probe":
						Expect(r.Header.Get("Range")).To(Equal("bytes=0-0"))
					}
					return response(r, 200, ""), nil
				}
				Expect(plugin.Execute(command, args(source))).To(Succeed())
				Expect(calls).To(Equal(expectedSuccess("HEAD "+source, "HEAD "+target, "GET "+target, "GET https://cdn.example.test/probe")))
			})
			It("rejects fallback redirects to HTTP before transport", func() {
				respond = func(r *http.Request) (*http.Response, error) {
					if r.Method == http.MethodHead {
						return response(r, 501, ""), nil
					}
					return response(r, 302, "http://example.test/insecure"), nil
				}
				Expect(plugin.Execute(command, args(source))).To(MatchError(ContainSubstring("only HTTPS")))
				Expect(calls).To(Equal([]string{"HEAD " + source, "GET " + source}))
			})
			It("preserves fallback transport failures", func() {
				failure := errors.New("fallback transport failure")
				respond = func(r *http.Request) (*http.Response, error) {
					if r.Method == http.MethodHead {
						return response(r, 405, ""), nil
					}
					return nil, failure
				}
				Expect(errors.Is(plugin.Execute(command, args(source)), failure)).To(BeTrue())
				Expect(calls).To(Equal([]string{"HEAD " + source, "GET " + source}))
			})
			It("bounds redirect loops", func() {
				respond = func(r *http.Request) (*http.Response, error) { return response(r, 302, source), nil }
				Expect(plugin.Execute(command, args(source))).To(MatchError(ContainSubstring("stopped after 10 redirects")))
				Expect(calls).To(HaveLen(10))
			})
			for _, phase := range []string{"HEAD", "fallback"} {
				It("propagates cancellation during "+phase, func() {
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					respond = func(r *http.Request) (*http.Response, error) {
						if phase == "fallback" && r.Method == http.MethodHead {
							return response(r, 501, ""), nil
						}
						cancel()
						select {
						case <-r.Context().Done():
							return nil, r.Context().Err()
						case <-time.After(time.Second):
							return nil, errors.New("request did not receive caller cancellation")
						}
					}
					Expect(errors.Is(plugin.ExecuteContext(ctx, command, args(source)), context.Canceled)).To(BeTrue())
					expected := []string{"HEAD " + source}
					if phase == "fallback" {
						expected = append(expected, "GET "+source)
					}
					Expect(calls).To(Equal(expected))
				})
			}
			It("does not start a request for an expired context", func() {
				ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
				defer cancel()
				Expect(errors.Is(plugin.ExecuteContext(ctx, command, args(source)), context.DeadlineExceeded)).To(BeTrue())
				Expect(calls).To(BeEmpty())
			})
			It("preserves transport failures", func() {
				failure := errors.New("fixture transport failure")
				respond = func(*http.Request) (*http.Response, error) { return nil, failure }
				Expect(errors.Is(plugin.Execute(command, args(source)), failure)).To(BeTrue())
				Expect(calls).To(Equal([]string{"HEAD " + source}))
			})
		})
	}
	It("enforces redirect policy again on the actual install download", func() {
		respond = func(r *http.Request) (*http.Response, error) {
			if r.Method == http.MethodGet {
				return response(r, 302, "http://example.test/insecure"), nil
			}
			return response(r, 200, ""), nil
		}
		Expect(plugin.Execute("install", []string{source, "tool", "--cli"})).To(MatchError(ContainSubstring("only HTTPS")))
		Expect(calls).To(Equal([]string{"HEAD " + source, "GET " + source}))
	})
	It("cancels the actual install download with the caller context", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		respond = func(r *http.Request) (*http.Response, error) {
			if r.Method == http.MethodHead {
				return response(r, 200, ""), nil
			}
			cancel()
			select {
			case <-r.Context().Done():
				return nil, r.Context().Err()
			case <-time.After(time.Second):
				return nil, errors.New("download did not receive caller cancellation")
			}
		}
		Expect(errors.Is(plugin.ExecuteContext(ctx, "install", []string{source, "tool", "--cli"}), context.Canceled)).To(BeTrue())
		Expect(calls).To(Equal([]string{"HEAD " + source, "GET " + source}))
	})
})
