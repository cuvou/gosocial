package cloudflare

import (
	"context"
	"fmt"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cloudflare/cloudflare-go"
)

// PurgeURL tells Cloudflare to purge a cached static file immediately from their CDN.
func PurgeURL(urls []string) error {

	log.Debug("PurgeURL: %+v", urls)

	if !config.Current.Cloudflare.Enabled {
		return nil
	}

	api, err := cloudflare.New(config.Current.Cloudflare.APIToken, config.Current.Cloudflare.Email)
	if err != nil {
		return fmt.Errorf("cloudflare.PurgeURL: error creating API client: %w", err)
	}

	ctx := context.Background()
	pcr := cloudflare.PurgeCacheRequest{
		Files: urls,
	}

	_, err = api.PurgeCache(ctx, config.Current.Cloudflare.ZoneID, pcr)
	if err != nil {
		return fmt.Errorf("cloudflare.PurgeURL: error from PurgeCache: %w", err)
	}

	return nil
}
