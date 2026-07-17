package http

import "smartping/src/g"

func validateConfig(config g.Config) error {
	return g.ValidateConfig(config)
}
