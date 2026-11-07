package main

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// XFCE backups are XML in the shape of xfconf's own per-channel files:
//
//	<xfce-config format="devex-xfce-backup" version="2">
//	  <channel name="xfwm4">
//	    <property name="/general/theme" type="string" value="Default"/>
//	    <property name="/general/workspace_names" type="array">
//	      <value type="string" value="Main"/>
//	    </property>
//	  </channel>
//	</xfce-config>
//
// Native XML supplies types and values missing from xfconf-query property reads.
// Older backups without types retain the existing property's type on restore.
const (
	xfceBackupFormat  = "devex-xfce-backup"
	xfceBackupVersion = 2
)

type xfceBackup struct {
	XMLName  xml.Name      `xml:"xfce-config"`
	Format   string        `xml:"format,attr,omitempty"`
	Version  int           `xml:"version,attr,omitempty"`
	Channels []xfceChannel `xml:"channel"`
}

type xfceChannel struct {
	Name       string         `xml:"name,attr"`
	Properties []xfceProperty `xml:"property"`
}

type xfceProperty struct {
	Name   string      `xml:"name,attr"`
	Type   string      `xml:"type,attr,omitempty"`
	Value  *string     `xml:"value,attr"`
	Values []xfceValue `xml:"value"`
}

type xfceValue struct {
	Type  string `xml:"type,attr"`
	Value string `xml:"value,attr"`
}

// xfconfScalarTypes are the value types xfconf-query accepts for -t.
var xfconfScalarTypes = []string{"string", "int", "uint", "bool", "double", "uchar", "char", "int16", "uint16", "int64", "uint64"}

const xfceOperationTimeout = 30 * time.Second

func xfconfQuery(ctx context.Context, args ...string) ([]byte, error) {
	return runXFCECommand(ctx, xfconfEnvironment(os.Environ()), "xfconf-query", args...)
}

// xfconfEnvironment pins only the numeric locale. xfconf-query calls
// setlocale(LC_ALL, "") and prints doubles with printf("%f"), which uses the
// locale's decimal separator, while -s parses them with g_ascii_strtod, which
// accepts only ".". LC_ALL=C is not used: g_print converts output to the
// locale charset, which would mangle non-ASCII strings. An LC_ALL override is
// re-expressed as LANG with no other LC_* variables, which selects the same
// locale for every other category. Sources: xfconf-query/main.c and
// common/xfconf-gvaluefuncs.c (xfconf 4.16.0 to master).
func xfconfEnvironment(environ []string) []string {
	all := ""
	for _, entry := range environ {
		if value, ok := strings.CutPrefix(entry, "LC_ALL="); ok {
			all = value
		}
	}
	env := make([]string, 0, len(environ)+2)
	for _, entry := range environ {
		key, _, _ := strings.Cut(entry, "=")
		if key == "LC_NUMERIC" || key == "LC_ALL" || (all != "" && (key == "LANG" || strings.HasPrefix(key, "LC_"))) {
			continue
		}
		env = append(env, entry)
	}
	if all != "" {
		env = append(env, "LANG="+all)
	}
	return append(env, "LC_NUMERIC=C")
}

func xfceCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	return runXFCECommand(ctx, nil, name, args...)
}

func runXFCECommand(ctx context.Context, env []string, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = env
	// Bound pipe draining as well, including children that inherit stdout/stderr.
	cmd.WaitDelay = time.Second
	output, err := cmd.Output()
	if err == nil {
		return output, nil
	}
	if ctx.Err() != nil {
		err = errors.Join(ctx.Err(), err)
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && len(bytes.TrimSpace(exit.Stderr)) > 0 {
		return output, fmt.Errorf("%w: %s", err, bytes.TrimSpace(exit.Stderr))
	}
	return output, err
}

// parseChannelListing reads `xfconf-query -l`: the first line is the header,
// translated by locale, so it is skipped by position; channel names follow,
// indented.
func parseChannelListing(output string) []string {
	lines := strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n")
	var channels []string
	for _, line := range lines[1:] {
		if channel := strings.TrimSpace(line); channel != "" {
			channels = append(channels, channel)
		}
	}
	return channels
}

// parsePropertyListing reads `xfconf-query -c <channel> -l`: one unindented
// property path per line. An empty channel prints a translated "contains no
// properties" sentence instead, which never starts with "/".
func parsePropertyListing(output string) []string {
	var properties []string
	for _, line := range strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n") {
		if property := strings.TrimSpace(line); strings.HasPrefix(property, "/") {
			properties = append(properties, property)
		}
	}
	return properties
}

// readXFCEProperty reads a property's value and, where xfconf reports it, type.
func readXFCEProperty(ctx context.Context, channel, property string) (xfceProperty, error) {
	plain, err := xfconfQuery(ctx, "-c", channel, "-p", property)
	if err != nil {
		return xfceProperty{}, err
	}
	verbose, err := xfconfQuery(ctx, "-c", channel, "-p", property, "-v")
	if err != nil {
		return xfceProperty{}, err
	}
	// Even equal scalar reads can hide a typed-looking string or an empty array.
	if string(plain) == string(verbose) {
		if native, nativeErr := readNativeXFCEProperty(channel, property); nativeErr == nil {
			if !nativeXFCEPropertyMatches(native, string(plain)) {
				return xfceProperty{}, errors.New("native xfconf XML does not match the current value; allow xfconfd to flush settings and retry backup")
			}
			return native, nil
		}
	}
	return decodeXFCEProperty(property, string(plain), string(verbose))
}

// The translated array header contains one decimal item count. Check it as
// well as the exact rendered body: embedded newlines mean body lines alone
// cannot establish an array's shape. Reject headers we cannot verify safely.
var xfceArrayCount = regexp.MustCompile(`[0-9]+`)

func nativeXFCEPropertyMatches(property xfceProperty, output string) bool {
	if property.Type != "array" {
		return property.Value != nil && output == *property.Value+"\n"
	}
	header, body, found := strings.Cut(output, "\n\n")
	if !found || strings.Contains(header, "\n") {
		return false
	}
	counts := xfceArrayCount.FindAllString(header, -1)
	if len(counts) != 1 || counts[0] != strconv.Itoa(len(property.Values)) {
		return false
	}
	var rendered strings.Builder
	for _, value := range property.Values {
		rendered.WriteString(value.Value)
		rendered.WriteByte('\n')
	}
	return body == rendered.String()
}

// decodeXFCEProperty compares plain and verbose reads. A typed read is
// accepted only when it is exactly the plain read with "[type] " prefixes, so
// a value that merely looks typed is never misread.
func decodeXFCEProperty(name, plain, verbose string) (xfceProperty, error) {
	plain = strings.TrimSuffix(plain, "\n")
	verbose = strings.TrimSuffix(verbose, "\n")

	if valueType, value, ok := splitTyped(verbose); ok && value == plain {
		return xfceProperty{Name: name, Type: valueType, Value: &value}, nil
	}

	// An array prints a translated count line and a blank line, then one item
	// per line.
	plainLines, verboseLines := strings.Split(plain, "\n"), strings.Split(verbose, "\n")
	if len(plainLines) > 2 && len(plainLines) == len(verboseLines) &&
		plainLines[0] == verboseLines[0] && plainLines[1] == "" && verboseLines[1] == "" {
		values := make([]xfceValue, 0, len(plainLines)-2)
		for i := 2; i < len(plainLines); i++ {
			valueType, value, ok := splitTyped(verboseLines[i])
			if !ok || value != plainLines[i] {
				values = nil
				break
			}
			values = append(values, xfceValue{Type: valueType, Value: value})
		}
		if values != nil {
			return xfceProperty{Name: name, Type: "array", Values: values}, nil
		}
	}

	if plain == verbose && !strings.Contains(plain, "\n") {
		return xfceProperty{Name: name, Value: &plain}, nil
	}
	return xfceProperty{}, errors.New("value has no type information and spans several lines, so it cannot be restored faithfully")
}

func splitTyped(line string) (string, string, bool) {
	if !strings.HasPrefix(line, "[") {
		return "", "", false
	}
	valueType, value, found := strings.Cut(line[1:], "] ")
	if !found || !slices.Contains(xfconfScalarTypes, valueType) {
		return "", "", false
	}
	return valueType, value, true
}

// parseXFCEBackup decodes and validates a backup before any setting is written.
func parseXFCEBackup(content []byte) (xfceBackup, error) {
	var backup xfceBackup
	decoder := xml.NewDecoder(bytes.NewReader(content))
	// Only an XML declaration, comments and whitespace may precede the root;
	// Decode alone would silently skip stray text.
	var root *xml.StartElement
	for root == nil {
		token, err := decoder.Token()
		if err != nil {
			return backup, fmt.Errorf("not an XFCE settings backup: %w", err)
		}
		switch value := token.(type) {
		case xml.StartElement:
			root = &value
		case xml.Comment:
		case xml.ProcInst:
			if value.Target != "xml" {
				return backup, errors.New("not an XFCE settings backup: unexpected processing instruction before the root element")
			}
		case xml.CharData:
			if strings.TrimSpace(string(value)) != "" {
				return backup, errors.New("not an XFCE settings backup: unexpected content before the root element")
			}
		default:
			return backup, errors.New("not an XFCE settings backup: unexpected content before the root element")
		}
	}
	if err := decoder.DecodeElement(&backup, root); err != nil {
		return backup, fmt.Errorf("not an XFCE settings backup: %w", err)
	}
	// DecodeElement reads one root element; reject another root or non-whitespace data.
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return backup, fmt.Errorf("invalid trailing XML: %w", err)
		}
		switch value := token.(type) {
		case xml.Comment:
		case xml.CharData:
			if strings.TrimSpace(string(value)) != "" {
				return backup, errors.New("unexpected content after XFCE backup")
			}
		default:
			return backup, errors.New("unexpected content after XFCE backup")
		}
	}
	if backup.Format != "" && backup.Format != xfceBackupFormat {
		return backup, fmt.Errorf("unsupported backup format %q", backup.Format)
	}
	if backup.Version > xfceBackupVersion {
		return backup, fmt.Errorf("backup version %d is newer than this plugin supports (%d)", backup.Version, xfceBackupVersion)
	}

	properties := 0
	seen := map[string]bool{}
	for _, channel := range backup.Channels {
		if channel.Name == "" || strings.HasPrefix(channel.Name, "-") || strings.ContainsFunc(channel.Name, isSpaceOrControl) {
			return backup, fmt.Errorf("invalid channel name %q", channel.Name)
		}
		for _, property := range channel.Properties {
			if err := validateXFCEProperty(property); err != nil {
				return backup, fmt.Errorf("channel %s: %w", channel.Name, err)
			}
			key := channel.Name + "\x00" + property.Name
			if seen[key] {
				return backup, fmt.Errorf("channel %s: property %s appears more than once", channel.Name, property.Name)
			}
			seen[key] = true
			properties++
		}
	}
	if properties == 0 {
		return backup, errors.New("no settings to restore; the backup has no properties (backups made before channel listing was fixed are empty), create a new one with 'devex desktop-xfce backup'")
	}
	return backup, nil
}

func validateXFCEProperty(property xfceProperty) error {
	if !strings.HasPrefix(property.Name, "/") || strings.ContainsFunc(property.Name, isSpaceOrControl) {
		return fmt.Errorf("invalid property name %q", property.Name)
	}
	switch {
	case property.Type == "array":
		if property.Value != nil {
			return fmt.Errorf("property %s: an array must not have a value attribute", property.Name)
		}
		for _, item := range property.Values {
			if !slices.Contains(xfconfScalarTypes, item.Type) {
				return fmt.Errorf("property %s: unsupported array item type %q", property.Name, item.Type)
			}
		}
	case property.Type == "" || slices.Contains(xfconfScalarTypes, property.Type):
		if property.Value == nil || len(property.Values) > 0 {
			return fmt.Errorf("property %s: a single value needs a value attribute and no <value> items", property.Name)
		}
	default:
		return fmt.Errorf("property %s: unsupported type %q", property.Name, property.Type)
	}
	return nil
}

func isSpaceOrControl(r rune) bool {
	return r <= ' ' || r == 0x7f
}

// restoreArgs builds the xfconf-query argv for one validated property. Typed
// values pass -n with -t so a missing property is created; untyped values keep
// the existing property's type.
func restoreArgs(channel string, property xfceProperty) []string {
	args := []string{"-c", channel, "-p", property.Name}
	switch property.Type {
	case "":
		return append(args, "-s", *property.Value)
	case "array":
		args = append(args, "-n", "-a")
		for _, item := range property.Values {
			args = append(args, "-t", item.Type, "-s", item.Value)
		}
		return args
	default:
		return append(args, "-n", "-t", property.Type, "-s", *property.Value)
	}
}

// Native per-channel XML uses nested relative property names. Search user
// overrides before system defaults, as xfconfd does; recover both values and
// types, since CLI line splitting cannot preserve embedded newlines in arrays.
// Source: xfconf 4.20.0 xfconfd/xfconf-backend-perchannel-xml.c.
func readNativeXFCEProperty(channel, name string) (xfceProperty, error) {
	if filepath.Base(channel) != channel || channel == "." || channel == ".." {
		return xfceProperty{}, errors.New("invalid native channel name")
	}
	config := os.Getenv("XDG_CONFIG_HOME")
	if config == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return xfceProperty{}, err
		}
		config = filepath.Join(home, ".config")
	}
	dirs := os.Getenv("XDG_CONFIG_DIRS")
	if dirs == "" {
		dirs = "/etc/xdg"
	}
	type nativeProperty struct {
		Name     string           `xml:"name,attr"`
		Type     string           `xml:"type,attr"`
		Value    *string          `xml:"value,attr"`
		Values   []xfceValue      `xml:"value"`
		Children []nativeProperty `xml:"property"`
	}
	for _, dir := range append([]string{config}, filepath.SplitList(dirs)...) {
		var native struct {
			XMLName    xml.Name         `xml:"channel"`
			Name       string           `xml:"name,attr"`
			Properties []nativeProperty `xml:"property"`
		}
		data, err := os.ReadFile(filepath.Join(dir, "xfce4", "xfconf", "xfce-perchannel-xml", channel+".xml"))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return xfceProperty{}, err
		}
		if err := xml.Unmarshal(data, &native); err != nil {
			return xfceProperty{}, err
		}
		if native.Name != channel {
			return xfceProperty{}, errors.New("native channel name mismatch")
		}
		var find func([]nativeProperty, string) (xfceProperty, bool)
		find = func(properties []nativeProperty, parent string) (xfceProperty, bool) {
			for _, prop := range properties {
				path := parent + "/" + prop.Name
				if path == name && prop.Type != "empty" {
					return xfceProperty{Name: name, Type: prop.Type, Value: prop.Value, Values: prop.Values}, true
				}
				if result, ok := find(prop.Children, path); ok {
					return result, true
				}
			}
			return xfceProperty{}, false
		}
		if result, ok := find(native.Properties, ""); ok {
			return result, validateXFCEProperty(result)
		}
	}
	return xfceProperty{}, errors.New("property absent from native xfconf XML")
}

// xfconf-query treats a missing -s as a read, even with -n -a. Xfconf's
// SetProperty(s,s,v) accepts an empty av; its GVariant conversion explicitly
// preserves zero children (common/xfconf-gvaluefuncs.c, xfconf 4.20.0).
func restoreXFCEProperty(ctx context.Context, channel string, property xfceProperty) error {
	if property.Type == "array" && len(property.Values) == 0 {
		_, err := xfceCommand(ctx, "gdbus", "call", "--session", "--dest", "org.xfce.Xfconf", "--object-path", "/org/xfce/Xfconf", "--method", "org.xfce.Xfconf.SetProperty", channel, property.Name, "<@av []>")
		return err
	}
	_, err := xfconfQuery(ctx, restoreArgs(channel, property)...)
	return err
}
