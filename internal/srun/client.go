package srun

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36 Edg/128.0.0.0"
)

var defaultHosts = []string{
	"https://login.hdu.edu.cn",
	"https://portal.hdu.edu.cn",
	"http://192.168.112.30",
	"http://192.168.112.97",
}

var ipPattern = regexp.MustCompile(`((1\d{2}|25[0-5]|2[0-4]\d|[1-9]?\d)\.){3}(25[0-5]|2[0-4]\d|1\d{2}|[1-9]?\d)`)

type Auth struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Logger interface {
	Logf(level string, format string, args ...any)
}

type noopLogger struct{}

func (noopLogger) Logf(string, string, ...any) {}

type Client struct {
	httpClient *http.Client
	logger     Logger
	rand       *rand.Rand

	Host     string
	Username string
	Password string

	acid   int
	n      string
	vtype  string
	encVer string
}

type infoPayload struct {
	Username string `json:"username"`
	Password string `json:"password"`
	IP       string `json:"ip"`
	ACID     string `json:"acid"`
	EncVer   string `json:"enc_ver"`
}

type challengeResponse struct {
	Challenge string `json:"challenge"`
	Error     string `json:"error"`
	ErrorMsg  string `json:"error_msg"`
}

func NewClient(username, password string, logger Logger) (*Client, error) {
	if logger == nil {
		logger = noopLogger{}
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	return &Client{
		httpClient: &http.Client{
			Jar:     jar,
			Timeout: 10 * time.Second,
		},
		logger:   logger,
		rand:     rand.New(rand.NewSource(time.Now().UnixNano())),
		Username: username,
		Password: password,
		n:        "200",
		vtype:    "1",
		encVer:   "srun_bx1",
	}, nil
}

func (c *Client) ensureHost(ctx context.Context) error {
	if c.Host != "" {
		return nil
	}
	for _, host := range defaultHosts {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, host, nil)
		if err != nil {
			c.logger.Logf("INFO", "Host %s %v", host, err)
			continue
		}
		req.Header.Set("User-Agent", userAgent)
		resp, err := c.httpClient.Do(req)
		if err != nil {
			c.logger.Logf("INFO", "Host %s %v", host, err)
			continue
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 500 {
			c.Host = host
			return nil
		}
		c.logger.Logf("INFO", "Host %s status %s", host, resp.Status)
	}
	return errors.New("failed to get host")
}

func (c *Client) get(ctx context.Context, path string, params url.Values) ([]byte, error) {
	if err := c.ensureHost(ctx); err != nil {
		return nil, err
	}

	u, err := url.Parse(c.Host + path)
	if err != nil {
		return nil, err
	}
	u.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 500 {
		return nil, fmt.Errorf("GET %s: %s", path, resp.Status)
	}
	return body, nil
}

func decodeJSONP(body []byte, callback string, out any) error {
	s := strings.TrimSpace(string(body))
	prefix := callback + "("
	if strings.HasPrefix(s, prefix) && strings.HasSuffix(s, ")") {
		s = s[len(prefix) : len(s)-1]
	} else {
		start := strings.IndexByte(s, '(')
		end := strings.LastIndexByte(s, ')')
		if start >= 0 && end > start {
			s = s[start+1 : end]
		}
	}
	return json.Unmarshal([]byte(s), out)
}

func nowMillis() int64 {
	return time.Now().UnixMilli()
}

func (c *Client) GetIP(ctx context.Context) (string, error) {
	var lastErr error
	for i := 0; i < 3; i++ {
		body, err := c.get(ctx, "/srun_portal_pc", nil)
		if err != nil {
			lastErr = err
		} else if ip := ipPattern.FindString(string(body)); ip != "" {
			return ip, nil
		} else {
			lastErr = errors.New("portal page did not contain IP")
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(time.Second):
		}
	}
	c.logger.Logf("ERROR", "Failed to get IP")
	return "", lastErr
}

func (c *Client) GetToken(ctx context.Context, ip string) (string, error) {
	callback := fmt.Sprintf("jQuery1124015280105355320628_%d", nowMillis())
	body, err := c.get(ctx, "/cgi-bin/get_challenge", url.Values{
		"callback": {callback},
		"username": {c.Username},
		"ip":       {ip},
		"_":        {fmt.Sprint(nowMillis())},
	})
	if err != nil {
		return "", err
	}

	var result challengeResponse
	if err := decodeJSONP(body, callback, &result); err != nil {
		return "", err
	}
	if result.Challenge == "" {
		if result.ErrorMsg != "" {
			return "", errors.New(result.ErrorMsg)
		}
		return "", errors.New("challenge is empty")
	}
	c.logger.Logf("INFO", "Token: %s", result.Challenge)
	return result.Challenge, nil
}

func (c *Client) getInfo(ip, token string) (string, error) {
	payload := infoPayload{
		Username: c.Username,
		Password: c.Password,
		IP:       ip,
		ACID:     fmt.Sprint(c.acid),
		EncVer:   c.encVer,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return "{SRBX1}" + customBase64Encode(xencode(raw, []byte(token))), nil
}

func (c *Client) getChecksum(ip, token, info string) string {
	checksum := token + c.Username
	checksum += token + hmacMD5(c.Password, token)
	checksum += token + fmt.Sprint(c.acid)
	checksum += token + ip
	checksum += token + c.n
	checksum += token + c.vtype
	checksum += token + info
	return sha1Hex(checksum)
}

func (c *Client) Login(ctx context.Context) (map[string]any, error) {
	for {
		ip, err := c.GetIP(ctx)
		if err != nil {
			return nil, err
		}
		token, err := c.GetToken(ctx, ip)
		if err != nil {
			return nil, err
		}
		info, err := c.getInfo(ip, token)
		if err != nil {
			return nil, err
		}
		checksum := c.getChecksum(ip, token, info)
		callback := fmt.Sprintf("jQuery1124015280105355320628_%d", nowMillis())
		device := devices[c.rand.Intn(len(devices))]

		body, err := c.get(ctx, "/cgi-bin/srun_portal", url.Values{
			"callback":     {callback},
			"action":       {"login"},
			"username":     {c.Username},
			"password":     {"{MD5}" + hmacMD5(c.Password, token)},
			"os":           {device[0]},
			"name":         {device[1]},
			"double_stack": {"0"},
			"chksum":       {checksum},
			"info":         {info},
			"ac_id":        {fmt.Sprint(c.acid)},
			"ip":           {ip},
			"n":            {c.n},
			"type":         {c.vtype},
			"_":            {fmt.Sprint(nowMillis())},
		})
		if err != nil {
			return nil, err
		}

		var result map[string]any
		if err := decodeJSONP(body, callback, &result); err != nil {
			return nil, err
		}

		if sucMsg := stringValue(result, "suc_msg"); sucMsg != "" {
			c.logger.Logf("SUCCESS", "login: %s %s %s %s", sucMsg, c.Username, c.Password, stringValue(result, "online_ip"))
			return result, nil
		}

		errCode := stringValue(result, "error")
		errMsg := stringValue(result, "error_msg")
		c.logger.Logf("ERROR", "%s: %s", errCode, errMsg)

		if strings.Contains(errMsg, "BAS") || strings.Contains(errMsg, "Nas") {
			c.logger.Logf("ERROR", "ac_id error, retry in 5 seconds...")
			c.acid++
			select {
			case <-ctx.Done():
				return result, ctx.Err()
			case <-time.After(5 * time.Second):
			}
			continue
		}

		if strings.Contains(errMsg, "E2901") {
			c.logger.Logf("ERROR", "username or password error...")
			result["error_msg"] = "4xx"
		} else if strings.Contains(errMsg, "E2606") {
			c.logger.Logf("ERROR", "user is disabled...")
			result["error_msg"] = "4xx"
		}

		return result, nil
	}
}

func (c *Client) Logout(ctx context.Context) (map[string]any, error) {
	status, err := c.Check(ctx)
	if err != nil {
		return nil, err
	}

	username := stringValue(status, "user_name")
	if username == "" {
		username = c.Username
	}
	ip := stringValue(status, "online_ip")
	if ip == "" {
		ip, err = c.GetIP(ctx)
		if err != nil {
			return nil, err
		}
	}

	callback := fmt.Sprintf("jQuery112405185119642573086_%d", nowMillis())
	t := time.Now().Unix()
	body, err := c.get(ctx, "/cgi-bin/rad_user_dm", url.Values{
		"callback": {callback},
		"username": {username},
		"ip":       {ip},
		"time":     {fmt.Sprint(t)},
		"unbind":   {"1"},
		"sign":     {sha1Hex(fmt.Sprintf("%d%s%s1%d", t, username, ip, t))},
		"_":        {fmt.Sprint(nowMillis())},
	})
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := decodeJSONP(body, callback, &result); err != nil {
		return nil, err
	}
	c.logger.Logf("INFO", "logout: %s", stringValue(result, "error"))
	return result, nil
}

func (c *Client) Check(ctx context.Context) (map[string]any, error) {
	callback := fmt.Sprintf("jQuery112405185119642573086_%d", nowMillis())
	body, err := c.get(ctx, "/cgi-bin/rad_user_info", url.Values{
		"callback": {callback},
		"_":        {fmt.Sprint(nowMillis())},
	})
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := decodeJSONP(body, callback, &result); err != nil {
		return nil, err
	}
	c.logger.Logf("INFO", "check: %s", stringValue(result, "error"))
	return result, nil
}

func stringValue(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case fmt.Stringer:
		return x.String()
	default:
		return fmt.Sprint(x)
	}
}
