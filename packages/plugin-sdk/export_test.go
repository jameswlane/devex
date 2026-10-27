package sdk

// ExportVerifyPluginSignature exports unexported verifyPluginSignature for testing
func (d *Downloader) ExportVerifyPluginSignature(pluginPath, downloadURL string) error {
	return d.verifyPluginSignature(pluginPath, downloadURL)
}

// ExportVerifyWithSystemGPG exports unexported verifyWithSystemGPG for testing
func (d *Downloader) ExportVerifyWithSystemGPG(pluginPath, signatureURL string) error {
	return d.verifyWithSystemGPG(pluginPath, signatureURL)
}
