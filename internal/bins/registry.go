package bins

// GetSpecs returns the list of available runtime installers initialized with the given DB.
func GetSpecs(db DB) []Installer {
	return []Installer{
		&JavaInstaller{},
		&GoInstaller{},
		&NodeInstaller{DB: db},
	}
}
