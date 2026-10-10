package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/egoist/mygo"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const issuer = "https://auth.openai.com"
const resource = "https://api.openai.com/v1"

type ChatGPTStatus struct {
	Connected bool   `json:"connected"`
	Pending   bool   `json:"pending"`
	Email     string `json:"email"`
	Model     string `json:"model"`
	Message   string `json:"message"`
}
type PlanModel struct {
	Slug        string `json:"slug"`
	DisplayName string `json:"display_name"`
	Visibility  string `json:"visibility"`
}
type registration struct {
	Host    string `json:"host"`
	Client  string `json:"client"`
	Subject string `json:"subject"`
}
type planTokens struct {
	Access  string `json:"access_token"`
	Refresh string `json:"refresh_token"`
	ID      string `json:"id_token"`
	Scope   string `json:"scope"`
	Type    string `json:"token_type"`
	Expires int    `json:"expires_in"`
}
type loginAttempt struct {
	state, nonce, verifier, redirect, client string
	claimed                                  bool
	server                                   *http.Server
	expires                                  time.Time
	cancel                                   context.CancelFunc
}
type ChatGPT struct {
	mu                    sync.Mutex
	reg                   registration
	tokens                planTokens
	expiry                time.Time
	pending               *loginAttempt
	email, model, message string
	models                []PlanModel
}

func randomValue() string {
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(data)
}
func registrationPath() (string, error) {
	dir, err := os.UserConfigDir()
	return filepath.Join(dir, "ParalinGo", "chatgpt-registration.json"), err
}
func writeRegistration(reg registration) error {
	path, err := registrationPath()
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, _ := json.Marshal(reg)
	file, err := os.CreateTemp(filepath.Dir(path), "registration-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
func (c *ChatGPT) Status() ChatGPTStatus {
	c.mu.Lock()
	defer c.mu.Unlock()
	return ChatGPTStatus{Connected: c.tokens.Access != "", Pending: c.pending != nil, Email: c.email, Model: c.model, Message: c.message}
}
func (c *ChatGPT) Login() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pending != nil {
		return fmt.Errorf("ログイン待ちです。公式ブラウザ画面を確認してください")
	}
	if c.reg.Host == "" {
		path, err := registrationPath()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err == nil {
			if json.Unmarshal(data, &c.reg) != nil {
				return fmt.Errorf("ログイン登録情報が壊れています")
			}
		} else if !os.IsNotExist(err) {
			return err
		}
		if c.reg.Host == "" {
			c.reg.Host = newHostID()
			if err = writeRegistration(c.reg); err != nil {
				return err
			}
		}
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("ログイン用のローカルポートを開けませんでした")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	attempt := &loginAttempt{state: randomValue(), nonce: randomValue(), verifier: randomValue(), redirect: "http://" + listener.Addr().String() + "/auth/callback", client: c.reg.Client, expires: time.Now().Add(10 * time.Minute), cancel: cancel}
	mux := http.NewServeMux()
	attempt.server = &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	mux.HandleFunc("/auth/callback", func(w http.ResponseWriter, r *http.Request) { c.callback(ctx, attempt, w, r) })
	c.pending = attempt
	c.message = "公式ブラウザでChatGPTログインとプラン利用の許可を完了してください。"
	go func() { _ = attempt.server.Serve(listener) }()
	go func() {
		<-ctx.Done()
		_ = attempt.server.Close()
		c.mu.Lock()
		if c.pending == attempt {
			c.pending = nil
			c.message = "ログインを終了しました。もう一度試してください。"
		}
		c.mu.Unlock()
	}()
	digest := sha256.Sum256([]byte(attempt.verifier))
	params := url.Values{"response_type": {"code"}, "redirect_uri": {attempt.redirect}, "scope": {"openid profile email offline_access resource.invoke chatgpt.tokens.use.direct"}, "resource": {resource}, "state": {attempt.state}, "nonce": {attempt.nonce}, "code_challenge_method": {"S256"}, "code_challenge": {base64.RawURLEncoding.EncodeToString(digest[:])}, "ext_agent_host_id": {c.reg.Host}}
	if attempt.client == "" {
		params.Set("client_id", "dynamic_agent_client")
		params.Set("agent_name_hint", "ParalinGo")
	} else {
		params.Set("client_id", attempt.client)
	}
	if err = mygo.Shell.OpenExternal(issuer + "/api/accounts/authorize?" + params.Encode()); err != nil {
		c.pending = nil
		cancel()
		return fmt.Errorf("公式ログイン画面を開けませんでした")
	}
	return nil
}
func validateCallback(a *loginAttempt, q url.Values) (string, error) {
	if time.Now().After(a.expires) || q.Get("state") == "" || q.Get("state") != a.state {
		return "", fmt.Errorf("ログイン要求の検証に失敗しました")
	}
	if q.Get("error") != "" {
		return "", fmt.Errorf("ログインが許可されませんでした")
	}
	client := q.Get("client_id")
	if a.client != "" {
		if client != "" && client != a.client {
			return "", fmt.Errorf("登録情報が一致しません")
		}
		client = a.client
	}
	if client == "" || client == "dynamic_agent_client" || q.Get("code") == "" {
		return "", fmt.Errorf("ログイン登録が未完了です")
	}
	return client, nil
}
func (c *ChatGPT) callback(ctx context.Context, a *loginAttempt, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	c.mu.Lock()
	if c.pending != a || a.claimed {
		c.mu.Unlock()
		http.Error(w, "Login expired", 400)
		return
	}
	client, err := validateCallback(a, r.URL.Query())
	if err != nil {
		c.mu.Unlock()
		http.Error(w, "Login validation failed. Return to ParalinGo.", 400)
		return
	}
	a.claimed = true
	c.message = "ログインを検証中…"
	c.mu.Unlock()
	defer a.cancel()
	tokens, err := exchangeTokens(ctx, url.Values{"grant_type": {"authorization_code"}, "client_id": {client}, "code": {r.URL.Query().Get("code")}, "code_verifier": {a.verifier}, "redirect_uri": {a.redirect}, "resource": {resource}})
	if err == nil {
		keys := oidc.NewRemoteKeySet(ctx, issuer+"/.well-known/jwks.json")
		verifier := oidc.NewVerifier(issuer, keys, &oidc.Config{ClientID: client})
		id, verifyErr := verifier.Verify(ctx, tokens.ID)
		err = verifyErr
		if err == nil {
			var claims struct {
				Nonce string `json:"nonce"`
				Email string `json:"email"`
			}
			err = id.Claims(&claims)
			if err == nil && (claims.Nonce != a.nonce || id.Subject == "" || !hasPlanScope(tokens.Scope)) {
				err = fmt.Errorf("プラン利用の許可を確認できませんでした")
			}
			c.mu.Lock()
			if err == nil && (ctx.Err() != nil || c.pending != a) {
				err = fmt.Errorf("ログインが取り消されました")
			}
			if err == nil && a.client != "" && c.reg.Subject != "" && c.reg.Subject != id.Subject {
				err = fmt.Errorf("ログインアカウントが一致しません")
			}
			if err == nil {
				reg := c.reg
				reg.Client = client
				reg.Subject = id.Subject
				err = writeRegistration(reg)
				if err == nil {
					c.pending = nil
					c.reg = reg
					c.tokens = tokens
					c.expiry = time.Now().Add(time.Duration(tokens.Expires) * time.Second)
					c.email = claims.Email
					c.model = ""
					c.models = nil
					c.message = "ChatGPTプランに接続しました。モデルを選択してください。"
				}
			}
			c.mu.Unlock()
		}
	}
	if err != nil {
		c.mu.Lock()
		if c.pending == a {
			c.pending = nil
			c.message = "ログインの検証に失敗しました。プラン利用の許可と対象プランを確認して再試行してください。"
		}
		c.mu.Unlock()
		http.Error(w, "Sign-in could not be verified. Return to ParalinGo.", 400)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, "<h1>ParalinGoに接続しました</h1><p>このタブを閉じてアプリへ戻り、モデルを選択してください。</p>")
}
func hasPlanScope(scope string) bool {
	for _, s := range strings.Fields(scope) {
		if s == "chatgpt.tokens.use.direct" {
			return true
		}
	}
	return false
}
func exchangeTokens(ctx context.Context, form url.Values) (planTokens, error) {
	req, _ := http.NewRequestWithContext(ctx, "POST", issuer+"/api/accounts/oauth/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := planHTTPClient().Do(req)
	if err != nil {
		return planTokens{}, fmt.Errorf("ログインサーバーに接続できませんでした")
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return planTokens{}, fmt.Errorf("ログイントークンを取得できませんでした")
	}
	var tokens planTokens
	if json.NewDecoder(io.LimitReader(response.Body, 1024*1024)).Decode(&tokens) != nil || tokens.Access == "" || tokens.Expires <= 0 || !strings.EqualFold(tokens.Type, "Bearer") {
		return tokens, fmt.Errorf("ログイン応答が正しくありません")
	}
	return tokens, nil
}
func planHTTPClient() *http.Client {
	return &http.Client{Timeout: 120 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
}
func (c *ChatGPT) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.tokens.Access == "" {
		return "", fmt.Errorf("ChatGPTでログインしてください")
	}
	if time.Now().Add(time.Minute).Before(c.expiry) {
		return c.tokens.Access, nil
	}
	if c.tokens.Refresh == "" {
		return "", fmt.Errorf("ログイン期限が切れました。再ログインしてください")
	}
	tokens, err := exchangeTokens(ctx, url.Values{"grant_type": {"refresh_token"}, "client_id": {c.reg.Client}, "refresh_token": {c.tokens.Refresh}, "resource": {resource}})
	if err != nil {
		return "", err
	}
	if tokens.Scope == "" {
		tokens.Scope = c.tokens.Scope
	}
	if !hasPlanScope(tokens.Scope) {
		return "", fmt.Errorf("ChatGPTプラン利用の許可がありません")
	}
	if tokens.Refresh == "" {
		tokens.Refresh = c.tokens.Refresh
	}
	c.tokens = tokens
	c.expiry = time.Now().Add(time.Duration(tokens.Expires) * time.Second)
	return tokens.Access, nil
}
func (c *ChatGPT) Models() ([]PlanModel, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	token, err := c.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", resource+"/models", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	response, err := planHTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("モデル一覧を取得できませんでした")
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, planError(response)
	}
	var catalog struct {
		Models []PlanModel `json:"models"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 1024*1024)).Decode(&catalog) != nil {
		return nil, fmt.Errorf("モデル一覧が正しくありません")
	}
	models := []PlanModel{}
	for _, model := range catalog.Models {
		if model.Visibility == "list" && model.Slug != "" {
			models = append(models, model)
		}
	}
	c.mu.Lock()
	c.models = models
	c.mu.Unlock()
	return models, nil
}
func (c *ChatGPT) SelectModel(slug string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, model := range c.models {
		if model.Slug == slug {
			c.model = slug
			return nil
		}
	}
	return fmt.Errorf("利用可能なモデルを選んでください")
}
func (c *ChatGPT) CancelLogin() {
	c.mu.Lock()
	a := c.pending
	c.pending = nil
	c.mu.Unlock()
	if a != nil {
		a.cancel()
	}
}
func (c *ChatGPT) Logout() string {
	c.mu.Lock()
	tokens := c.tokens
	client := c.reg.Client
	a := c.pending
	c.pending = nil
	c.tokens = planTokens{}
	c.model = ""
	c.models = nil
	c.email = ""
	c.message = "ログアウトしました。"
	c.mu.Unlock()
	if a != nil {
		a.cancel()
	}
	if tokens.Refresh != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		req, _ := http.NewRequestWithContext(ctx, "GET", issuer+"/.well-known/openid-configuration", nil)
		response, err := planHTTPClient().Do(req)
		if err == nil {
			var config struct {
				Revoke string `json:"revocation_endpoint"`
			}
			_ = json.NewDecoder(io.LimitReader(response.Body, 65536)).Decode(&config)
			response.Body.Close()
			u, _ := url.Parse(config.Revoke)
			if u != nil && u.Scheme == "https" && u.Host == "auth.openai.com" {
				form := url.Values{"token": {tokens.Refresh}, "token_type_hint": {"refresh_token"}, "client_id": {client}}
				req, _ = http.NewRequestWithContext(ctx, "POST", u.String(), strings.NewReader(form.Encode()))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				response, err = planHTTPClient().Do(req)
				if err == nil {
					response.Body.Close()
					if response.StatusCode == 200 {
						return "ログアウトしました。"
					}
				}
			}
		}
		return "ローカルではログアウトしました。接続解除を確認できなかったため、ChatGPT設定からParalinGoを解除してください。"
	}
	return "ログアウトしました。"
}
func planError(response *http.Response) error {
	if response.StatusCode == 429 {
		return fmt.Errorf("ChatGPTプランの利用上限に達したか、一時的に制限されています。生成を停止しました。ChatGPT設定の利用状況を確認してください (HTTP 429)")
	}
	if response.StatusCode == 401 || response.StatusCode == 403 {
		return fmt.Errorf("ChatGPTプランの認証・利用許可を確認できません。再ログインしてください (HTTP %d)", response.StatusCode)
	}
	return fmt.Errorf("ChatGPTプランへの接続に失敗しました (HTTP %d)", response.StatusCode)
}

func newHostID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("urn:uuid:%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}
