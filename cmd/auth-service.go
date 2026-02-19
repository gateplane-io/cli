// Copyright (C) 2026 Ioannis Torakis <john.torakis@gmail.com>
// SPDX-License-Identifier: Elastic-2.0
//
// Licensed under the Elastic License 2.0.
// You may obtain a copy of the license at:
// https://www.elastic.co/licensing/elastic-license
//
// Use, modification, and redistribution permitted under the terms of the license,
// except for providing this software as a commercial service or product.

package main

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gateplane-io/client-cli/internal/config"
	"github.com/gateplane-io/client-cli/internal/service"
	"github.com/gateplane-io/client-cli/internal/vault"
	"github.com/gateplane-io/client-cli/pkg/models"
	vault_api "github.com/hashicorp/vault/api"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

func serviceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "service",
		Aliases: []string{"svc"},
		Short:   "GatePlane Service authentication operations",
		Long:    "Manage authentication with GatePlane Services",
	}

	cmd.AddCommand(
		serviceLoginCmd(),
		serviceLogoutCmd(),
		serviceStatusCmd(),
	)

	return cmd
}

func serviceLoginCmd() *cobra.Command {
	var (
		clientID    string
		skipBrowser bool
	)

	cmd := &cobra.Command{
		Use:     "login",
		Aliases: []string{"signin"},
		Short:   "Authenticate with GatePlane service",
		Long:    "Authenticate with GatePlane service using OIDC to obtain a JWT token",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.GetConfig()

			// Use client ID from config if not provided via flag
			if clientID == "" {
				clientID = cfg.Service.ClientID
			}

			// Client ID is required
			if clientID == "" {
				return fmt.Errorf("client ID is required. Use --client-id flag or set it in config")
			}

			// Save service configuration
			cfg.Service.ClientID = clientID
			// if err := config.SaveConfig(); err != nil {
			// 	return fmt.Errorf("failed to save service configuration: %w", err)
			// }

			// Create vault client for OIDC authentication
			vaultConfig := getVaultClientConfig()
			client, err := vault.NewClient(vaultConfig)
			if err != nil {
				return wrapError("create vault client", err)
			}

			// Perform OIDC login to get JWT
			jwt, err := performOIDCLogin(client.VaultClient(), clientID, skipBrowser)
			if err != nil {
				return wrapError("OIDC login", err)
			}

			// Save JWT to config
			cfg.Service.JWT = jwt
			if err := config.SaveConfig(); err != nil {
				return wrapError("save authentication data", err)
			}

			printSuccessMessage("Successfully authenticated with GatePlane Services")
			return nil
		},
	}

	cmd.Flags().StringVar(&clientID, "client-id", "", "OIDC client ID")
	cmd.Flags().BoolVar(&skipBrowser, "skip-browser", false, "Skip opening browser for OIDC")

	return cmd
}

func serviceLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "logout",
		Aliases: []string{"signout"},
		Short:   "Clear service authentication",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.ClearServiceAuth(); err != nil {
				return wrapError("clear service auth", err)
			}

			fmt.Println("Logged out from GatePlane Services")
			return nil
		},
	}
}

func serviceStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "status",
		Aliases: []string{"whoami"},
		Short:   "Check service authentication status",
		RunE: func(cmd *cobra.Command, args []string) error {
			svcClient, err := createServiceClient()
			if err != nil {
				fmt.Println("Not authenticated with GatePlane Services (using Community Edition features)")
				return nil
			}

			fmt.Printf("Service Address: %s\n", config.ServiceAddress)

			// Test the JWT by making a request to /ping
			if err := svcClient.Ping(); err != nil {
				fmt.Printf("Authentication status: Invalid/Expired (%s)\n", err)
			} else {
				fmt.Println("Authentication status: Valid")
			}

			// Test Notifications
			if err := svcClient.SendNotification(&models.RequestServiceResponse{}, service.Test); err != nil {
				fmt.Printf("Notification status: Failed (%s)\n", err)
			} else {
				fmt.Println("Notification status: Working!")
			}

			return nil
		},
	}
}

func CreateWrappedToken(client *vault_api.Client) (string, error) {
	// Request wrapping for the specific operation/path.
	client.SetWrappingLookupFunc(func(operation, path string) string {
		if (operation == "POST" || operation == "PUT") && path == "auth/token/create" {
			return "1m" // desired wrap TTL
		}
		return ""
	})

	secret, err := client.Auth().Token().Create(&vault_api.TokenCreateRequest{
		// NumUses: 1,
	})

	if err != nil {
		return "", err
	}
	if secret == nil || secret.WrapInfo == nil {
		return "", fmt.Errorf("no wrap_info in response - %v", secret)
	}

	return secret.WrapInfo.Token, nil
}

func performOIDCLogin(client *vault_api.Client, clientID string, skipBrowser bool) (string, error) {
	vaultAddr := client.Address()
	redirectURI := "http://localhost:45450/oidc/callback"

	wrappedToken, err := CreateWrappedToken(client)
	autoLoginParams := ""
	if err != nil {
		fmt.Printf("Could not create wrapped token for auto-login (%s)\n", err)
	} else {
		fmt.Printf("Generated Wrapped Token for auto-login\n")
		autoLoginParams = fmt.Sprintf("?wrapped_token=%s&with=token", wrappedToken)
	}

	// Configure OAuth2 with PKCE support
	config := &oauth2.Config{
		ClientID:    clientID,
		RedirectURL: redirectURI,
		Scopes:      []string{"openid", "profile", "messenger_options"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  fmt.Sprintf("%s/ui/vault/identity/oidc/provider/gateplane/authorize%s", vaultAddr, autoLoginParams),
			TokenURL: fmt.Sprintf("%s/v1/identity/oidc/provider/gateplane/token", vaultAddr),
		},
	}

	// Use PKCE
	verifier := oauth2.GenerateVerifier()
	authURL := config.AuthCodeURL("state", oauth2.S256ChallengeOption(verifier))

	var authCode string
	var authError error
	var wg sync.WaitGroup

	if !skipBrowser {
		// Start callback server
		server, serverCh := startCallbackServer("45450")
		defer func() {
			_ = server.Shutdown(context.Background())
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case result := <-serverCh:
				if result.Error != nil {
					authError = result.Error
				} else {
					authCode = result.Code
				}
			case <-time.After(5 * time.Minute): // Timeout after 5 minutes
				authError = fmt.Errorf("authentication timed out")
			}
		}()

		fmt.Printf("Starting local callback server on port 45450...\n")
		fmt.Printf("Opening browser for OIDC authentication...\n")
		fmt.Printf("If browser doesn't open automatically, visit: %s\n", authURL)

		if err := browser.OpenURL(authURL); err != nil {
			fmt.Printf("Failed to open browser: %v\n", err)
			fmt.Printf("Please visit the URL manually: %s\n", authURL)
		}

		fmt.Printf("Waiting for callback...\n")
		wg.Wait()

		if authError != nil {
			return "", authError
		}
	} else {
		// Manual code input
		fmt.Printf("Visit this URL in your browser: %s\n", authURL)
		fmt.Print("Enter the authorization code from the callback URL: ")
		if _, err := fmt.Scanln(&authCode); err != nil {
			return "", wrapError("read authorization code", err)
		}
	}

	if authCode == "" {
		return "", fmt.Errorf("no authorization code received")
	}

	return exchangeCodeForToken(config, authCode, verifier)
}

type callbackResult struct {
	Code  string
	State string
	Error error
}

func startCallbackServer(port string) (*http.Server, <-chan callbackResult) {
	resultCh := make(chan callbackResult, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/oidc/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		state := r.URL.Query().Get("state")
		errorParam := r.URL.Query().Get("error")
		errorDesc := r.URL.Query().Get("error_description")

		if errorParam != "" {
			msg := fmt.Sprintf("OIDC error: %s", errorParam)
			if errorDesc != "" {
				msg += fmt.Sprintf(" - %s", errorDesc)
			}
			resultCh <- callbackResult{Error: fmt.Errorf("%s", msg)}
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprintf(w, "<html><body><h1>Authentication Failed</h1><p>%s</p><p>You can close this window.</p></body></html>", msg)
			return
		}

		if code == "" {
			resultCh <- callbackResult{Error: fmt.Errorf("no authorization code received")}
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprintf(w, "<html><body><h1>Authentication Failed</h1><p>No authorization code received</p><p>You can close this window.</p></body></html>")
			return
		}

		resultCh <- callbackResult{Code: code, State: state}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "<html><body><h1>Authentication Successful</h1><p>You can close this window and return to the CLI.</p><script>setTimeout(window.close, 5000);</script></body></html>")
	})

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			resultCh <- callbackResult{Error: wrapError("callback server", err)}
		}
	}()

	return server, resultCh
}

// exchangeCodeForToken exchanges authorization code for OIDC token
func exchangeCodeForToken(config *oauth2.Config, authCode, verifier string) (string, error) {
	ctx := context.Background()

	// Add debugging context with custom HTTP client
	httpClient := &http.Client{
		Timeout:   30 * time.Second,
		Transport: http.DefaultTransport,
		// Transport: &debugTransport{http.DefaultTransport},
	}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, httpClient)

	token, err := config.Exchange(ctx, authCode, oauth2.VerifierOption(verifier))
	if err != nil {
		return "", wrapError("exchange code for token", err)
	}

	fmt.Printf("Token response received: AccessToken present: %v, TokenType: %s\n",
		token.AccessToken != "", token.TokenType)

	// Get the ID token from the extra fields
	idToken, ok := token.Extra("id_token").(string)
	if !ok || idToken == "" {
		// Print all extra fields for debugging
		fmt.Printf("Available extra fields: %+v\n", token.Extra(""))
		return "", fmt.Errorf("no ID token received from OIDC provider")
	}

	return idToken, nil
}
