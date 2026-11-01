package sdk

import (
	"context"
	"io"
)

// PluginProgressStage identifies a plugin download lifecycle stage.
type PluginProgressStage string

const (
	PluginProgressDownloading PluginProgressStage = "downloading"
	PluginProgressVerifying   PluginProgressStage = "verifying"
	PluginProgressDone        PluginProgressStage = "done"
	PluginProgressError       PluginProgressStage = "error"
)

// PluginProgressEvent reports progress for one plugin download.
type PluginProgressEvent struct {
	PluginName      string
	Stage           PluginProgressStage
	BytesDownloaded int64
	TotalBytes      int64
	Err             error
}

// PluginProgressCallback receives plugin download lifecycle events.
type PluginProgressCallback func(PluginProgressEvent)

// DownloadRequiredPluginsWithContextAndProgress downloads required plugins with an optional,
// synchronous callback. The callback is scoped to this operation, including retries.
func (d *Downloader) DownloadRequiredPluginsWithContextAndProgress(ctx context.Context, requiredPlugins []string, progress PluginProgressCallback) error {
	if progress == nil {
		return d.DownloadRequiredPluginsWithContext(ctx, requiredPlugins)
	}
	finished := make(map[string]bool)
	report := PluginProgressCallback(func(event PluginProgressEvent) {
		if event.Stage == PluginProgressDone || event.Stage == PluginProgressError {
			finished[event.PluginName] = true
		}
		progress(event)
	})
	err := d.DownloadRequiredPluginsWithContext(context.WithValue(ctx, pluginProgressKey{}, report), requiredPlugins)
	if err != nil {
		// An early failure (for example registry unavailability) also settles the
		// plugins that could not be attempted, so callers never leave them pending.
		for _, name := range requiredPlugins {
			if !finished[name] {
				progress(PluginProgressEvent{PluginName: name, Stage: PluginProgressDownloading})
				progress(PluginProgressEvent{PluginName: name, Stage: PluginProgressError, Err: err})
			}
		}
	}
	return err
}

// A private context key keeps progress scoped to one download without changing
// existing downloader methods or introducing shared mutable callback state.
type pluginProgressKey struct{}

func pluginProgressFromContext(ctx context.Context) PluginProgressCallback {
	progress, _ := ctx.Value(pluginProgressKey{}).(PluginProgressCallback)
	return progress
}

func reportPluginProgress(progress PluginProgressCallback, event PluginProgressEvent) {
	if progress != nil {
		progress(event)
	}
}

type pluginProgressWriter struct {
	writer     io.Writer
	pluginName string
	total      int64
	downloaded int64
	progress   PluginProgressCallback
}

func (w *pluginProgressWriter) Write(p []byte) (int, error) {
	n, err := w.writer.Write(p)
	w.downloaded += int64(n)
	if n > 0 {
		reportPluginProgress(w.progress, PluginProgressEvent{
			PluginName:      w.pluginName,
			Stage:           PluginProgressDownloading,
			BytesDownloaded: w.downloaded,
			TotalBytes:      w.total,
		})
	}
	return n, err
}
