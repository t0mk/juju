// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package model

import (
	"strings"

	"github.com/juju/cmd"
	"github.com/juju/errors"
	"launchpad.net/gnuflag"

	"github.com/juju/juju/cmd/juju/block"
	"github.com/juju/juju/cmd/modelcmd"
)

type accessCommand struct {
	modelcmd.ControllerCommandBase

	User        string
	ModelNames  []string
	ModelAccess string
}

func (c *accessCommand) SetFlags(f *gnuflag.FlagSet) {
	f.StringVar(&c.ModelAccess, "acl", "read", "access control")
}

func (c *accessCommand) Init(args []string) (err error) {
	if len(args) < 1 {
		return errors.New("no user specified")
	}

	if len(args) < 2 {
		return errors.New("no model specified")
	}

	c.User = args[0]
	c.ModelNames = args[1:]
	return nil
}

func (c *accessCommand) modelUUIDs(ctx *cmd.Context) ([]string, error) {
	var result []string
	store := c.ClientStore()
	controllerName := c.ControllerName()
	accountName := c.AccountName()
	for _, modelName := range c.ModelNames {
		model, err := store.ModelByName(controllerName, accountName, modelName)
		if errors.IsNotFound(err) {
			// The model isn't known locally, so query the models available in the controller.
			ctx.Verbosef("model %q not cached locally, refreshing models from controller", modelName)
			if err := c.RefreshModels(store, controllerName, accountName); err != nil {
				return nil, errors.Annotatef(err, "refreshing model %q", modelName)
			}
			model, err = store.ModelByName(controllerName, accountName, modelName)
		}
		if err != nil {
			return nil, errors.Annotatef(err, "model %q not found", modelName)
		}
		result = append(result, model.ModelUUID)
	}
	return result, nil
}

const grantModelHelpDoc = `
Grant another user access to a model.

Examples:
 juju grant joe model1
     Grant user "joe" default (read) access to the current model

 juju grant joe model1 --acl=write
     Grant user "joe" write access to the current model

 juju grant sam model1 model2
     Grant user "sam" default (read) access to two models named "model1" and "model2".
 `

func NewGrantCommand() cmd.Command {
	return modelcmd.WrapController(&grantCommand{})
}

// grantCommand represents the command to grant a user access to one or more models.
type grantCommand struct {
	accessCommand
	api GrantModelAPI
}

// Info implements Command.Info.
func (c *grantCommand) Info() *cmd.Info {
	return &cmd.Info{
		Name:    "grant",
		Args:    "<user> <model1> [<model2> .. <modelN>]",
		Purpose: "grant another user access to the given models",
		Doc:     strings.TrimSpace(grantModelHelpDoc),
	}
}

func (c *grantCommand) getAPI() (GrantModelAPI, error) {
	if c.api != nil {
		return c.api, nil
	}
	return c.NewModelManagerAPIClient()
}

// GrantModelAPI defines the API functions used by the grant command.
type GrantModelAPI interface {
	Close() error
	GrantModel(user, access string, modelUUIDs ...string) error
}

func (c *grantCommand) Run(ctx *cmd.Context) error {
	client, err := c.getAPI()
	if err != nil {
		return err
	}
	defer client.Close()

	models, err := c.modelUUIDs(ctx)
	if err != nil {
		return err
	}
	return block.ProcessBlockedError(client.GrantModel(c.User, c.ModelAccess, models...), block.BlockChange)
}

const revokeModelHelpDoc = `
Deny a user access to an model that was previously shared with them.

Revoking read access also revokes write access.

Examples:
 juju revoke joe model1
     Revoke read access from user "joe" for model "model1".

 juju revoke joe model1 model2 --acl=write
     Revoke write access from user "joe" for models "model1" and "model2".
`

func NewRevokeCommand() cmd.Command {
	return modelcmd.WrapController(&revokeCommand{})
}

// revokeCommand revokes a user's access to models.
type revokeCommand struct {
	accessCommand
	api RevokeModelAPI
}

func (c *revokeCommand) Info() *cmd.Info {
	return &cmd.Info{
		Name:    "revoke",
		Args:    "<user> <model1> [<model2> .. <modelN>]",
		Purpose: "revoke user access to models",
		Doc:     strings.TrimSpace(revokeModelHelpDoc),
	}
}

func (c *revokeCommand) getAPI() (RevokeModelAPI, error) {
	if c.api != nil {
		return c.api, nil
	}
	return c.NewModelManagerAPIClient()
}

// RevokeModelAPI defines the API functions used by the revoke command.
type RevokeModelAPI interface {
	Close() error
	RevokeModel(user, access string, modelUUIDs ...string) error
}

func (c *revokeCommand) Run(ctx *cmd.Context) error {
	client, err := c.getAPI()
	if err != nil {
		return err
	}
	defer client.Close()

	modelUUIDs, err := c.modelUUIDs(ctx)
	if err != nil {
		return err
	}
	return block.ProcessBlockedError(client.RevokeModel(c.User, c.ModelAccess, modelUUIDs...), block.BlockChange)
}
