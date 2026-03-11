package provider

import (
	"context"
	"fmt"
	"strings"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

type SmtpCredential struct{}

type SmtpCredentialArgs struct {
	Domain   string `pulumi:"domain"`
	Username string `pulumi:"username"`
	Password string `pulumi:"password" provider:"secret"`
}

type SmtpCredentialState struct {
	SmtpCredentialArgs
	Created int64 `pulumi:"created"`
}

func (s *SmtpCredential) Annotate(a infer.Annotator) {
	a.Describe(&s, "Manages an ImprovMX SMTP credential for sending email.")
}

func (s *SmtpCredentialArgs) Annotate(a infer.Annotator) {
	a.Describe(&s.Domain, "The domain this SMTP credential belongs to.")
	a.Describe(&s.Username, "The SMTP username.")
	a.Describe(&s.Password, "The SMTP password.")
}

func (s *SmtpCredentialState) Annotate(a infer.Annotator) {
	a.Describe(&s.Created, "Unix timestamp when the credential was created.")
}

func makeCredentialID(domain, username string) string { return domain + "/" + username }

func parseCredentialID(id string) (string, string) {
	parts := strings.SplitN(id, "/", 2)
	return parts[0], parts[1]
}

func (SmtpCredential) Create(ctx context.Context, req infer.CreateRequest[SmtpCredentialArgs]) (infer.CreateResponse[SmtpCredentialState], error) {
	input := req.Inputs
	id := makeCredentialID(input.Domain, input.Username)
	if req.DryRun {
		return infer.CreateResponse[SmtpCredentialState]{
			ID:     id,
			Output: SmtpCredentialState{SmtpCredentialArgs: input},
		}, nil
	}

	client, err := getClient(ctx)
	if err != nil {
		return infer.CreateResponse[SmtpCredentialState]{}, err
	}
	cred, err := client.CreateSmtpCredential(input.Domain, input.Username, input.Password)
	if err != nil {
		if apiErr, ok := err.(*APIError); ok && apiErr.IsAlreadyExists() {
			// Credential already exists — adopt it and update password.
			if updateErr := client.UpdateSmtpCredential(input.Domain, input.Username, input.Password); updateErr != nil {
				return infer.CreateResponse[SmtpCredentialState]{}, fmt.Errorf("updating adopted SMTP credential: %w", updateErr)
			}
			creds, listErr := client.ListSmtpCredentials(input.Domain)
			if listErr != nil {
				return infer.CreateResponse[SmtpCredentialState]{}, fmt.Errorf("adopting existing SMTP credential: %w", listErr)
			}
			for _, c := range creds {
				if c.Username == input.Username {
					cred = &c
					break
				}
			}
			if cred == nil {
				return infer.CreateResponse[SmtpCredentialState]{}, fmt.Errorf("SMTP credential %s not found after adopt", input.Username)
			}
		} else {
			return infer.CreateResponse[SmtpCredentialState]{}, fmt.Errorf("creating SMTP credential: %w", err)
		}
	}

	return infer.CreateResponse[SmtpCredentialState]{
		ID: id,
		Output: SmtpCredentialState{
			SmtpCredentialArgs: input,
			Created:            cred.Created,
		},
	}, nil
}

func (SmtpCredential) Read(ctx context.Context, req infer.ReadRequest[SmtpCredentialArgs, SmtpCredentialState]) (infer.ReadResponse[SmtpCredentialArgs, SmtpCredentialState], error) {
	domain, username := parseCredentialID(req.ID)
	client, err := getClient(ctx)
	if err != nil {
		return infer.ReadResponse[SmtpCredentialArgs, SmtpCredentialState]{}, err
	}

	creds, err := client.ListSmtpCredentials(domain)
	if err != nil {
		if apiErr, ok := err.(*APIError); ok && apiErr.IsNotFound() {
			return infer.ReadResponse[SmtpCredentialArgs, SmtpCredentialState]{ID: ""}, nil
		}
		return infer.ReadResponse[SmtpCredentialArgs, SmtpCredentialState]{}, fmt.Errorf("reading SMTP credentials: %w", err)
	}

	for _, c := range creds {
		if c.Username == username {
			args := SmtpCredentialArgs{
				Domain:   domain,
				Username: c.Username,
				Password: req.State.Password,
			}
			return infer.ReadResponse[SmtpCredentialArgs, SmtpCredentialState]{
				ID:     req.ID,
				Inputs: args,
				State: SmtpCredentialState{
					SmtpCredentialArgs: args,
					Created:            c.Created,
				},
			}, nil
		}
	}

	// Credential not found — signal deletion.
	return infer.ReadResponse[SmtpCredentialArgs, SmtpCredentialState]{ID: ""}, nil
}

func (SmtpCredential) Update(ctx context.Context, req infer.UpdateRequest[SmtpCredentialArgs, SmtpCredentialState]) (infer.UpdateResponse[SmtpCredentialState], error) {
	input := req.Inputs
	if req.DryRun {
		return infer.UpdateResponse[SmtpCredentialState]{
			Output: SmtpCredentialState{SmtpCredentialArgs: input, Created: req.State.Created},
		}, nil
	}

	domain, username := parseCredentialID(req.ID)
	client, err := getClient(ctx)
	if err != nil {
		return infer.UpdateResponse[SmtpCredentialState]{}, err
	}
	if err := client.UpdateSmtpCredential(domain, username, input.Password); err != nil {
		return infer.UpdateResponse[SmtpCredentialState]{}, fmt.Errorf("updating SMTP credential: %w", err)
	}

	return infer.UpdateResponse[SmtpCredentialState]{
		Output: SmtpCredentialState{
			SmtpCredentialArgs: input,
			Created:            req.State.Created,
		},
	}, nil
}

func (SmtpCredential) Delete(ctx context.Context, req infer.DeleteRequest[SmtpCredentialState]) (infer.DeleteResponse, error) {
	domain, username := parseCredentialID(req.ID)
	client, err := getClient(ctx)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	if err := client.DeleteSmtpCredential(domain, username); err != nil {
		if apiErr, ok := err.(*APIError); ok && apiErr.IsNotFound() {
			return infer.DeleteResponse{}, nil
		}
		return infer.DeleteResponse{}, fmt.Errorf("deleting SMTP credential: %w", err)
	}
	return infer.DeleteResponse{}, nil
}

func (SmtpCredential) Diff(ctx context.Context, req infer.DiffRequest[SmtpCredentialArgs, SmtpCredentialState]) (infer.DiffResponse, error) {
	diff := map[string]p.PropertyDiff{}
	if req.Inputs.Domain != req.State.Domain {
		diff["domain"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}
	if req.Inputs.Username != req.State.Username {
		diff["username"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}
	if req.Inputs.Password != req.State.Password {
		diff["password"] = p.PropertyDiff{Kind: p.Update}
	}
	_, domainReplace := diff["domain"]
	_, usernameReplace := diff["username"]
	return infer.DiffResponse{
		DeleteBeforeReplace: domainReplace || usernameReplace,
		HasChanges:          len(diff) > 0,
		DetailedDiff:        diff,
	}, nil
}
