package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/pkg/browser"

	"github.com/gateplane-io/client-cli/internal/config"
	"github.com/gateplane-io/client-cli/internal/vault"
	vault_api "github.com/hashicorp/vault/api"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
	"golang.org/x/term"
)

func authCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authentication operations",
		Long:  "Manage authentication with Vault/OpenBao",
	}

	cmd.AddCommand(
		authLoginCmd(),
		authStatusCmd(),
		authLogoutCmd(),
		serviceCmd(),
	)

	return cmd
}

func authLoginCmd() *cobra.Command {
	var (
		vaultAddr string
		namespace string
		token     string
	)

	cmd := &cobra.Command{
		Use:     "login",
		Aliases: []string{"signin"},
		Short:   "Authenticate with Vault",
		Long:    "Authenticate with Vault/OpenBao using a token",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.GetConfig()

			// Get vault address
			if vaultAddr == "" {
				vaultAddr = cfg.Vault.Address
				if vaultAddr == "" {
					if addr := os.Getenv("VAULT_ADDR"); addr != "" {
						vaultAddr = addr
					} else {
						fmt.Print("Enter Vault address: ")
						if _, err := fmt.Scanln(&vaultAddr); err != nil {
							return fmt.Errorf("failed to read vault address: %w", err)
						}
					}
				}
			}

			// Token-based authentication
			if token == "" {
				fmt.Print("Enter Vault token: ")
				tokenBytes, err := term.ReadPassword(int(syscall.Stdin))
				if err != nil {
					return fmt.Errorf("failed to read token: %w", err)
				}
				fmt.Println()
				token = string(tokenBytes)
			}

			// Update config
			cfg.Vault.Address = vaultAddr
			cfg.Vault.Token = token
			if namespace != "" {
				cfg.Vault.Namespace = namespace
			}

			// Test connection
			vaultConfig := &vault.Config{
				Address:   vaultAddr,
				Token:     token,
				Namespace: namespace,
			}

			client, err := vault.NewClient(vaultConfig)
			if err != nil {
				return fmt.Errorf("failed to create vault client: %w", err)
			}

			// Try to get token info to verify auth
			tokenInfo, err := client.VaultClient().Auth().Token().LookupSelf()
			if err != nil {
				return fmt.Errorf("authentication failed: %w", err)
			}

			// Save config
			if err := config.SaveConfig(); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			printAuthSuccessMessage(tokenInfo)

			return nil
		},
	}

	cmd.Flags().StringVar(&vaultAddr, "address", "", "Vault address")
	cmd.Flags().StringVar(&namespace, "namespace", "", "Vault namespace")
	cmd.Flags().StringVar(&token, "token", "", "Vault token (use with caution)")

	return cmd
}

func authStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "status",
		Aliases: []string{"whoami"},
		Short:   "Check authentication status",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.GetConfig()

			client, err := createVaultClient()
			if err != nil {
				return wrapError("create vault client", err)
			}

			fmt.Printf("Vault Address: %s\n", cfg.Vault.Address)
			if cfg.Vault.Namespace != "" {
				fmt.Printf("Namespace: %s\n", cfg.Vault.Namespace)
			}

			tokenInfo, err := client.VaultClient().Auth().Token().LookupSelf()
			if err != nil {
				printFailedMessage("Not authenticated")
				return nil
			}

			printTokenInfo(tokenInfo)
			printSuccessMessage("Authenticated")
			return nil
		},
	}
}

func authLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "logout",
		Aliases: []string{"signout"},
		Short:   "Clear stored authentication",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.GetConfig()
			cfg.Vault.Token = ""

			if err := config.SaveConfig(); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			fmt.Println("Logged out successfully")
			return nil
		},
	}
}

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
				return fmt.Errorf("failed to create vault client: %w", err)
			}

			// Get JWKS from Vault OIDC provider
			jwks, err := getJWKS(client.VaultClient())
			if err != nil {
				return fmt.Errorf("failed to get JWKS: %w", err)
			}

			// Perform OIDC login to get JWT
			jwt, err := performOIDCLogin(client.VaultClient(), clientID, skipBrowser)
			if err != nil {
				return fmt.Errorf("OIDC login failed: %w", err)
			}

			// Save JWT and JWKS to config
			cfg.Service.JWT = jwt
			cfg.Service.JWKS = jwks
			if err := config.SaveConfig(); err != nil {
				return fmt.Errorf("failed to save authentication data: %w", err)
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
				return fmt.Errorf("failed to clear service auth: %w", err)
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
			cfg := config.GetConfig()

			if cfg.Service.JWT == "" {
				fmt.Println("Not authenticated with GatePlane Services (using Community Edition features)")
				return nil
			}

			fmt.Printf("Service Address: %s\n", config.ServiceAddress)

			// Test the JWT by making a request to /ping
			if err := testServiceAuth(cfg.Service.JWT, cfg.Service.JWKS); err != nil {
				fmt.Printf("Authentication status: Invalid/Expired (%s)\n", err)
			} else {
				fmt.Println("Authentication status: Valid")
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
		return "", fmt.Errorf("no wrap_info in response - %s", secret)
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
			return "", fmt.Errorf("failed to read authorization code: %w", err)
		}
	}

	if authCode == "" {
		return "", fmt.Errorf("no authorization code received")
	}

	return exchangeCodeForToken(config, authCode, verifier)
}

func getJWKS(client *vault_api.Client) (string, error) {
	// Get JWKS from the Vault OIDC provider via HTTP
	vaultAddr := client.Address()
	jwksURL := fmt.Sprintf("%s/v1/identity/oidc/provider/gateplane/.well-known/keys", vaultAddr)

	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Get(jwksURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch JWKS from %s: %w", jwksURL, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("JWKS request failed with status %d: %s", resp.StatusCode, string(body))
	}

	jwksData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read JWKS response: %w", err)
	}

	return string(jwksData), nil
}

func testServiceAuth(jwt, jwks string) error {
	client := &http.Client{Timeout: 10 * time.Second}

	// Create request body with both JWT and JWKS for PCRE
	requestBody := map[string]string{
		"jwt":  jwt,
		"jwks": jwks,
	}

	bodyJSON, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequest("POST", config.ServiceAddress+"/api/ping", bytes.NewBuffer(bodyJSON))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwt)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != 200 {
		// Try to read error message
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("service returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
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
		_, _ = fmt.Fprintf(w, "<html><body><h1>Authentication Successful</h1><p>You can close this window and return to the CLI.</p></body></html>")
	})

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			resultCh <- callbackResult{Error: fmt.Errorf("callback server error: %w", err)}
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
		return "", fmt.Errorf("failed to exchange code for token: %w", err)
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

// printAuthSuccessMessage prints authentication success with optional username
func printAuthSuccessMessage(tokenInfo *vault_api.Secret) {
	if tokenInfo == nil || tokenInfo.Data == nil {
		fmt.Println("Successfully authenticated")
		return
	}

	username, ok := tokenInfo.Data["display_name"].(string)
	if !ok || username == "" {
		fmt.Println("Successfully authenticated")
		return
	}

	fmt.Printf("Successfully authenticated as %s\n", username)
}

// printTokenInfo prints token information in a formatted way
func printTokenInfo(tokenInfo *vault_api.Secret) {
	if tokenInfo == nil || tokenInfo.Data == nil {
		return
	}

	if username, ok := tokenInfo.Data["display_name"].(string); ok && username != "" {
		fmt.Printf("Authenticated as: %s\n", username)
	}

	policies, ok := tokenInfo.Data["policies"].([]interface{})
	if !ok {
		return
	}

	fmt.Print("Policies: ")
	for i, p := range policies {
		if i > 0 {
			fmt.Print(", ")
		}
		fmt.Print(p)
	}
	fmt.Println()
}

// debugTransport wraps an http.RoundTripper to log requests and responses
type debugTransport struct {
	rt http.RoundTripper
}

func (d *debugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	fmt.Printf("Making request to: %s %s\n", req.Method, req.URL.String())

	if req.Body != nil {
		bodyBytes, _ := io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		fmt.Printf("Request body: %s\n", string(bodyBytes))
	}

	resp, err := d.rt.RoundTrip(req)
	if err != nil {
		fmt.Printf("Request failed: %v\n", err)
		return resp, err
	}

	fmt.Printf("Response status: %d\n", resp.StatusCode)

	if resp.Body != nil {
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		fmt.Printf("Response body: %s\n", string(bodyBytes))
	}

	return resp, err
}
