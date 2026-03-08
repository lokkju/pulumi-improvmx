package provider

import (
	"context"
	"fmt"
	"os"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

type Domain struct{}

type DomainArgs struct {
	Domain            string  `pulumi:"domain"`
	NotificationEmail *string `pulumi:"notificationEmail,optional"`
	Webhook           *string `pulumi:"webhook,optional"`
}

type DomainState struct {
	DomainArgs
	Active  bool   `pulumi:"active"`
	Display string `pulumi:"display"`
}

func (d *Domain) Annotate(a infer.Annotator) {
	a.Describe(&d, "Manages an ImprovMX domain for email forwarding.")
}

func (d *DomainArgs) Annotate(a infer.Annotator) {
	a.Describe(&d.Domain, "The domain name to register with ImprovMX.")
	a.Describe(&d.NotificationEmail, "Email address for delivery notifications.")
	a.Describe(&d.Webhook, "Webhook URL for delivery notifications.")
}

func (d *DomainState) Annotate(a infer.Annotator) {
	a.Describe(&d.Active, "Whether the domain's DNS is correctly configured.")
	a.Describe(&d.Display, "Display name of the domain.")
}

func getClient(ctx context.Context) *ImprovMXClient {
	config := infer.GetConfig[ProviderConfig](ctx)
	token := config.ApiToken
	if token == "" {
		token = os.Getenv("IMPROVMX_API_TOKEN")
	}
	if token == "" {
		p.GetLogger(ctx).Errorf("ImprovMX API token not configured")
	}
	return NewImprovMXClient(token)
}

func (Domain) Create(ctx context.Context, req infer.CreateRequest[DomainArgs]) (infer.CreateResponse[DomainState], error) {
	input := req.Inputs
	if req.DryRun {
		return infer.CreateResponse[DomainState]{
			ID:     input.Domain,
			Output: DomainState{DomainArgs: input},
		}, nil
	}

	client := getClient(ctx)
	notifEmail := ""
	if input.NotificationEmail != nil {
		notifEmail = *input.NotificationEmail
	}

	domain, err := client.CreateDomain(input.Domain, notifEmail)
	if err != nil {
		if apiErr, ok := err.(*APIError); ok && apiErr.IsAlreadyExists() {
			// Domain already registered — adopt and sync optional fields.
			fields := map[string]string{}
			if input.NotificationEmail != nil {
				fields["notification_email"] = *input.NotificationEmail
			}
			if input.Webhook != nil {
				fields["webhook"] = *input.Webhook
			}
			if len(fields) > 0 {
				domain, err = client.UpdateDomain(input.Domain, fields)
				if err != nil {
					return infer.CreateResponse[DomainState]{}, fmt.Errorf("updating adopted domain: %w", err)
				}
			} else {
				domain, err = client.GetDomain(input.Domain)
				if err != nil {
					return infer.CreateResponse[DomainState]{}, fmt.Errorf("adopting existing domain: %w", err)
				}
			}
		} else {
			return infer.CreateResponse[DomainState]{}, fmt.Errorf("creating domain: %w", err)
		}
	}

	return infer.CreateResponse[DomainState]{
		ID: domain.Domain,
		Output: DomainState{
			DomainArgs: input,
			Active:     domain.Active,
			Display:    domain.Display,
		},
	}, nil
}

func (Domain) Read(ctx context.Context, req infer.ReadRequest[DomainArgs, DomainState]) (infer.ReadResponse[DomainArgs, DomainState], error) {
	client := getClient(ctx)
	domain, err := client.GetDomain(req.ID)
	if err != nil {
		if apiErr, ok := err.(*APIError); ok && apiErr.IsNotFound() {
			return infer.ReadResponse[DomainArgs, DomainState]{ID: ""}, nil
		}
		return infer.ReadResponse[DomainArgs, DomainState]{}, fmt.Errorf("reading domain: %w", err)
	}

	args := DomainArgs{
		Domain: domain.Domain,
	}
	if domain.NotificationEmail != "" {
		args.NotificationEmail = &domain.NotificationEmail
	}
	if domain.Webhook != "" {
		args.Webhook = &domain.Webhook
	}

	return infer.ReadResponse[DomainArgs, DomainState]{
		ID:     domain.Domain,
		Inputs: args,
		State: DomainState{
			DomainArgs: args,
			Active:     domain.Active,
			Display:    domain.Display,
		},
	}, nil
}

func (Domain) Update(ctx context.Context, req infer.UpdateRequest[DomainArgs, DomainState]) (infer.UpdateResponse[DomainState], error) {
	input := req.Inputs
	if req.DryRun {
		return infer.UpdateResponse[DomainState]{
			Output: DomainState{DomainArgs: input},
		}, nil
	}

	client := getClient(ctx)
	fields := map[string]string{}
	if input.NotificationEmail != nil {
		fields["notification_email"] = *input.NotificationEmail
	}
	if input.Webhook != nil {
		fields["webhook"] = *input.Webhook
	}

	domain, err := client.UpdateDomain(req.ID, fields)
	if err != nil {
		return infer.UpdateResponse[DomainState]{}, fmt.Errorf("updating domain: %w", err)
	}

	return infer.UpdateResponse[DomainState]{
		Output: DomainState{
			DomainArgs: input,
			Active:     domain.Active,
			Display:    domain.Display,
		},
	}, nil
}

func (Domain) Delete(ctx context.Context, req infer.DeleteRequest[DomainState]) (infer.DeleteResponse, error) {
	client := getClient(ctx)
	if err := client.DeleteDomain(req.ID); err != nil {
		if apiErr, ok := err.(*APIError); ok && apiErr.IsNotFound() {
			return infer.DeleteResponse{}, nil
		}
		return infer.DeleteResponse{}, fmt.Errorf("deleting domain: %w", err)
	}
	return infer.DeleteResponse{}, nil
}

func (Domain) Diff(ctx context.Context, req infer.DiffRequest[DomainArgs, DomainState]) (infer.DiffResponse, error) {
	diff := map[string]p.PropertyDiff{}

	if req.Inputs.Domain != req.State.Domain {
		diff["domain"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}
	if ptrDiffers(req.Inputs.NotificationEmail, req.State.NotificationEmail) {
		diff["notificationEmail"] = p.PropertyDiff{Kind: p.Update}
	}
	if ptrDiffers(req.Inputs.Webhook, req.State.Webhook) {
		diff["webhook"] = p.PropertyDiff{Kind: p.Update}
	}

	return infer.DiffResponse{
		DeleteBeforeReplace: true,
		HasChanges:          len(diff) > 0,
		DetailedDiff:        diff,
	}, nil
}

func ptrDiffers(a, b *string) bool {
	if a == nil && b == nil {
		return false
	}
	if a == nil || b == nil {
		return true
	}
	return *a != *b
}
