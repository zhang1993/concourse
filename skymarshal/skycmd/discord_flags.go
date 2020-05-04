package skycmd

import (
	"encoding/json"
	"errors"

	"github.com/concourse/dex/connector/discord"
	"github.com/concourse/flag"
	multierror "github.com/hashicorp/go-multierror"
)

func init() {
	RegisterConnector(&Connector{
		id:         "discord",
		config:     &DiscordFlags{},
		teamConfig: &DiscordTeamFlags{},
	})
}

type DiscordFlags struct {
	ClientID           string      `long:"client-id" description:"(Required) Client id"`
	ClientSecret       string      `long:"client-secret" description:"(Required) Client secret"`
	CACerts            []flag.File `long:"ca-cert" description:"CA Certificate"`
	InsecureSkipVerify bool        `long:"skip-ssl-validation" description:"Skip SSL validation"`
}

func (flag *DiscordFlags) Name() string {
	return "Discord"
}

func (flag *DiscordFlags) Validate() error {
	var errs *multierror.Error

	if flag.ClientID == "" {
		errs = multierror.Append(errs, errors.New("Missing client-id"))
	}

	if flag.ClientSecret == "" {
		errs = multierror.Append(errs, errors.New("Missing client-secret"))
	}

	return errs.ErrorOrNil()
}

func (flag *DiscordFlags) Serialize(redirectURI string) ([]byte, error) {
	if err := flag.Validate(); err != nil {
		return nil, err
	}

	caCerts := []string{}
	for _, file := range flag.CACerts {
		caCerts = append(caCerts, file.Path())
	}

	return json.Marshal(discord.Config{
		ClientID:           flag.ClientID,
		ClientSecret:       flag.ClientSecret,
		RootCAs:            caCerts,
		InsecureSkipVerify: flag.InsecureSkipVerify,
		RedirectURI:        redirectURI,
	})
}

type DiscordTeamFlags struct {
	Users  []string `long:"user" description:"A whitelisted Discord user" value-name:"USERNAME"`
	Guilds []string `long:"guild" description:"A whitelisted Discord org" value-name:"GUILD_NAME"`
	Roles  []string `long:"role" description:"A whitelisted Discord role" value-name:"GUILD_NAME:ROLE_NAME"`
}

func (flag *DiscordTeamFlags) GetUsers() []string {
	return flag.Users
}

func (flag *DiscordTeamFlags) GetGroups() []string {
	return append(flag.Guilds, flag.Roles...)
}
