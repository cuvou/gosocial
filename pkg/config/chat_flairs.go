package config

import "github.com/cuvou/gosocial/pkg/chat/flairs"

// Custom flair configs for the BareRTC chat room.
var (
	TestFlair = flairs.Flair{
		Name:  "Flaired",
		Color: "danger",
		Icon:  "fa fa-fire",
	}

	ShyAccountFlair = flairs.Flair{
		Name:  "Shy Account",
		Color: "private",
		Icon:  "fa fa-ghost",
	}

	BirthdayFlair = flairs.Flair{
		Name:  "It's my birthday!",
		Color: "info",
		Icon:  "fa fa-cake-candles",
	}
)
