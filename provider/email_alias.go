package provider

import (
	"context"
	"fmt"
	"strings"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

type EmailAlias struct{}

type EmailAliasArgs struct {
	Domain  string `pulumi:"domain"`
	Alias   string `pulumi:"alias"`
	Forward string `pulumi:"forward"`
}

type EmailAliasState struct {
	EmailAliasArgs
}

func (e *EmailAlias) Annotate(a infer.Annotator) {
	a.Describe(&e, "Manages an ImprovMX email alias (forwarding rule).")
}

func (e *EmailAliasArgs) Annotate(a infer.Annotator) {
	a.Describe(&e.Domain, "The domain this alias belongs to.")
	a.Describe(&e.Alias, "The alias name (e.g., 'info', '*' for catch-all).")
	a.Describe(&e.Forward, "Comma-separated destination email addresses.")
}

func makeAliasID(domain, alias string) string { return domain + "/" + alias }

func parseAliasID(id string) (string, string) {
	parts := strings.SplitN(id, "/", 2)
	return parts[0], parts[1]
}

func (EmailAlias) Create(ctx context.Context, req infer.CreateRequest[EmailAliasArgs]) (infer.CreateResponse[EmailAliasState], error) {
	input := req.Inputs
	id := makeAliasID(input.Domain, input.Alias)
	if req.DryRun {
		return infer.CreateResponse[EmailAliasState]{ID: id, Output: EmailAliasState{EmailAliasArgs: input}}, nil
	}

	client := getClient(ctx)
	alias, err := client.CreateAlias(input.Domain, input.Alias, input.Forward)
	if err != nil {
		return infer.CreateResponse[EmailAliasState]{}, fmt.Errorf("creating alias: %w", err)
	}

	return infer.CreateResponse[EmailAliasState]{
		ID: id,
		Output: EmailAliasState{EmailAliasArgs: EmailAliasArgs{
			Domain:  input.Domain,
			Alias:   alias.Alias,
			Forward: alias.Forward,
		}},
	}, nil
}

func (EmailAlias) Read(ctx context.Context, req infer.ReadRequest[EmailAliasArgs, EmailAliasState]) (infer.ReadResponse[EmailAliasArgs, EmailAliasState], error) {
	domain, aliasName := parseAliasID(req.ID)
	client := getClient(ctx)
	alias, err := client.GetAlias(domain, aliasName)
	if err != nil {
		return infer.ReadResponse[EmailAliasArgs, EmailAliasState]{}, fmt.Errorf("reading alias: %w", err)
	}

	args := EmailAliasArgs{Domain: domain, Alias: alias.Alias, Forward: alias.Forward}
	return infer.ReadResponse[EmailAliasArgs, EmailAliasState]{
		ID: req.ID, Inputs: args, State: EmailAliasState{EmailAliasArgs: args},
	}, nil
}

func (EmailAlias) Update(ctx context.Context, req infer.UpdateRequest[EmailAliasArgs, EmailAliasState]) (infer.UpdateResponse[EmailAliasState], error) {
	input := req.Inputs
	if req.DryRun {
		return infer.UpdateResponse[EmailAliasState]{Output: EmailAliasState{EmailAliasArgs: input}}, nil
	}

	domain, aliasName := parseAliasID(req.ID)
	client := getClient(ctx)
	alias, err := client.UpdateAlias(domain, aliasName, input.Forward)
	if err != nil {
		return infer.UpdateResponse[EmailAliasState]{}, fmt.Errorf("updating alias: %w", err)
	}

	return infer.UpdateResponse[EmailAliasState]{
		Output: EmailAliasState{EmailAliasArgs: EmailAliasArgs{
			Domain: domain, Alias: alias.Alias, Forward: alias.Forward,
		}},
	}, nil
}

func (EmailAlias) Delete(ctx context.Context, req infer.DeleteRequest[EmailAliasState]) (infer.DeleteResponse, error) {
	domain, aliasName := parseAliasID(req.ID)
	client := getClient(ctx)
	if err := client.DeleteAlias(domain, aliasName); err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("deleting alias: %w", err)
	}
	return infer.DeleteResponse{}, nil
}

func (EmailAlias) Diff(ctx context.Context, req infer.DiffRequest[EmailAliasArgs, EmailAliasState]) (infer.DiffResponse, error) {
	diff := map[string]p.PropertyDiff{}
	if req.Inputs.Domain != req.State.Domain {
		diff["domain"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}
	if req.Inputs.Alias != req.State.Alias {
		diff["alias"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}
	if req.Inputs.Forward != req.State.Forward {
		diff["forward"] = p.PropertyDiff{Kind: p.Update}
	}
	return infer.DiffResponse{
		DeleteBeforeReplace: true,
		HasChanges:          len(diff) > 0,
		DetailedDiff:        diff,
	}, nil
}
