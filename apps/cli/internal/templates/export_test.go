package templates

import "context"

// Export internal import helpers for the external Ginkgo security specifications.
var CopyTemplateArchive = copyTemplateArchive

func (ctm *CustomTemplateManager) InstallLocalForTest(path string) (*CustomTemplateManifest, string, error) {
	return ctm.installFromLocal(path, &TemplateSource{Type: "local", Path: path})
}

func (ctm *CustomTemplateManager) ExtractZipForTest(path, dest string) error {
	return ctm.extractZip(path, dest)
}

func (ctm *CustomTemplateManager) InstallHTTPForTest(ctx context.Context, ref string, source *TemplateSource) (*CustomTemplateManifest, string, error) {
	return ctm.installFromHTTP(ctx, ref, source)
}

func (ctm *CustomTemplateManager) RegisterTemplateForTest(manifest *CustomTemplateManifest) error {
	return ctm.registerTemplate(manifest)
}
