// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package auth

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/x/mongo/driver/auth/creds"
	"go.mongodb.org/mongo-driver/x/mongo/driver/auth/internal/awsv4"
)

// MongoDBAWS is the mechanism name for MongoDBAWS.
const MongoDBAWS = "MONGODB-AWS"

func newMongoDBAWSAuthenticator(cred *Cred) (Authenticator, error) {
	if cred.Source != "" && cred.Source != "$external" {
		return nil, newAuthError("MONGODB-AWS source must be empty or $external", nil)
	}
	return &MongoDBAWSAuthenticator{
		source:       cred.Source,
		username:     cred.Username,
		password:     cred.Password,
		sessionToken: cred.Props["AWS_SESSION_TOKEN"],
	}, nil
}

// MongoDBAWSAuthenticator uses AWS-IAM credentials over SASL to authenticate a connection.
type MongoDBAWSAuthenticator struct {
	source       string
	username     string
	password     string
	sessionToken string
}

type mongoDBAWSAuthenticator struct {
	creds.AwsCredentials
	creds.AwsCredentialProvider
}

// GetCredentials gets AWS credentials.
func (a *mongoDBAWSAuthenticator) getCredentials(ctx context.Context) (*awsv4.StaticProvider, error) {
	c, err := a.AwsCredentials.ValidateAndMakeCredentials()
	if c != nil || err != nil {
		return c, err
	}
	return a.AwsCredentialProvider.GetCredentials(ctx)
}

// Auth authenticates the connection.
func (a *MongoDBAWSAuthenticator) Auth(ctx context.Context, cfg *Config) error {
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		return errors.New("cfg.HTTPClient must not be nil")
	}
	adapter := &awsSaslAdapter{
		conversation: &awsConversation{
			provider: &mongoDBAWSAuthenticator{
				AwsCredentials: creds.AwsCredentials{
					Username: a.username,
					Password: a.password,
					Token:    a.sessionToken,
				},
				AwsCredentialProvider: creds.AwsCredentialProvider{
					HTTPClient: httpClient,
				},
			},
		},
	}
	err := ConductSaslConversation(ctx, cfg, a.source, adapter)
	if err != nil {
		return newAuthError("sasl conversation error", err)
	}
	return nil
}

type awsSaslAdapter struct {
	conversation *awsConversation
}

var _ SaslClient = (*awsSaslAdapter)(nil)

func (a *awsSaslAdapter) Start() (string, []byte, error) {
	step, err := a.conversation.Step(nil)
	if err != nil {
		return MongoDBAWS, nil, err
	}
	return MongoDBAWS, step, nil
}

func (a *awsSaslAdapter) Next(challenge []byte) ([]byte, error) {
	step, err := a.conversation.Step(challenge)
	if err != nil {
		return nil, err
	}
	return step, nil
}

func (a *awsSaslAdapter) Completed() bool {
	return a.conversation.Done()
}
