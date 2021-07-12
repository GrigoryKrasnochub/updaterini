package updaterini

func (uc *UpdateConfig) CheckForUpdates() {
	var versions []Version
	for _, source := range uc.Sources {
		sVersions, _ := source.getSourceVersions(uc.ApplicationConfig) // TODO add err handler
		versions = append(versions, sVersions...)
	}

	getLatestVersion(uc.ApplicationConfig, versions)
}
