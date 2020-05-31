package plugin

import (
	"os"
	"strings"

	"code.cloudfoundry.org/lager"
)

func detectPluginEnv(logger lager.Logger) []string {
	env := os.Environ()

	var pluginEnv []string
	for _, e := range env {
		spl := strings.SplitN(e, "=", 2)
		if len(spl) != 2 {
			logger.Info("bogus-env", lager.Data{"env": spl})
			continue
		}

		name := spl[0]
		if !strings.HasPrefix(name, pluginEnvPrefix) {
			continue
		}

		pluginEnv = append(pluginEnv, e)

		logger.Info("forwarding-eventstore-plugin-env-var", lager.Data{
			"env": name,
		})
	}
	return pluginEnv
}