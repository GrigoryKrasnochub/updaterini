package updaterini

type Channel struct {
	Name     string // for searching in update files name
	Priority int    // bigger value more preferable
}

type ApplicationConfig struct {
	CurrentVersion string
}

type UpdateConfig struct {
	ApplicationConfig ApplicationConfig
	CurrentChannel    Channel
	Channels          []Channel
	Sources           []UpdateSource // would be used in slice order
}

func NewUpdateConfig() UpdateConfig {
	channels := []Channel{
		{
			Name:     "dev",
			Priority: 0,
		},
		{
			Name:     "alpha",
			Priority: 1,
		},
		{
			Name:     "beta",
			Priority: 2,
		},
	}

	return UpdateConfig{
		ApplicationConfig: ApplicationConfig{
			"1.0.0",
		},
		CurrentChannel: Channel{},
		Channels:       channels,
		Sources:        nil,
	}
}


