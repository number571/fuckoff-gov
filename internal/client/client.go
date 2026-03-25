package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/number571/fuckoff-gov/internal/consts"
	"github.com/number571/fuckoff-gov/internal/models"
	"github.com/number571/go-peer/pkg/crypto/asymmetric"
)

type sClient struct {
	authToken  string
	addr       string
	pkHash     string
	httpClient *http.Client
	privKey    asymmetric.IPrivKey
}

func NewClient(addr string, privKey asymmetric.IPrivKey, httpClient *http.Client) IClient {
	return &sClient{
		addr:       addr,
		pkHash:     privKey.GetPubKey().GetHasher().ToString(),
		privKey:    privKey,
		httpClient: httpClient,
	}
}

func (p *sClient) Auth(ctx context.Context) error {
	authTask, err := p.getAuthTask(ctx)
	if err != nil {
		return err
	}
	authToken, err := p.signAuthRandBytes(ctx, authTask)
	if err != nil {
		return err
	}
	p.authToken = authToken
	return nil
}

func (p *sClient) HasAuth(_ context.Context) bool {
	return p.authToken != ""
}

func (p *sClient) getAuthTask(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf(p.addr+"/auth?pkhash=%s", url.QueryEscape(p.pkHash)),
		nil,
	)
	if err != nil {
		return "", err
	}
	rsp, err := p.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status code: %d (ping)", rsp.StatusCode)
	}
	authTask := rsp.Header.Get(consts.HeaderAuthTask)
	if authTask == "" {
		return "", errors.New("auth task is nil")
	}
	return authTask, nil
}

func (p *sClient) signAuthRandBytes(ctx context.Context, authTask string) (string, error) {
	sign := p.privKey.GetDSAPrivKey().SignBytes([]byte(authTask))
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		fmt.Sprintf(p.addr+"/auth?pkhash=%s", url.QueryEscape(p.pkHash)),
		bytes.NewReader(sign),
	)
	if err != nil {
		return "", err
	}
	req.Header.Set(consts.HeaderAuthTask, authTask)
	rsp, err := p.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status code: %d (ping)", rsp.StatusCode)
	}
	authToken := rsp.Header.Get(consts.HeaderAuthToken)
	if authTask == "" {
		return "", errors.New("auth task is nil")
	}
	return authToken, nil
}

func (p *sClient) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.addr+"/ping", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Auth-Token", p.authToken)
	rsp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status code: %d (ping)", rsp.StatusCode)
	}
	return nil
}

func (p *sClient) InitClient(ctx context.Context, clientInfo *models.ClientInfo) error {
	reqBody, err := json.Marshal(clientInfo)
	if err != nil {
		panic(err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.addr+"/client/init", bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	req.Header.Set("Auth-Token", p.authToken)
	rsp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status code: %d (initClient)", rsp.StatusCode)
	}
	return err
}

func (p *sClient) LoadClient(ctx context.Context, pkhash string) (*models.ClientInfo, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf(p.addr+"/client/load?pkhash=%s", url.QueryEscape(pkhash)),
		nil,
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Auth-Token", p.authToken)
	rsp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status code: %d (loadClient)", rsp.StatusCode)
	}
	clientInfo := &models.ClientInfo{}
	if err := json.NewDecoder(rsp.Body).Decode(clientInfo); err != nil {
		return nil, err
	}
	return clientInfo, nil
}

func (p *sClient) CountChannels(ctx context.Context) (uint64, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		p.addr+"/client/channels/size",
		nil,
	)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Auth-Token", p.authToken)
	rsp, err := p.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("bad status code: %d (countChannels)", rsp.StatusCode)
	}
	rspBody, err := io.ReadAll(rsp.Body)
	if err != nil {
		return 0, err
	}
	size, err := strconv.ParseUint(string(rspBody), 10, 64)
	if err != nil {
		return 0, err
	}
	return size, nil
}

func (p *sClient) ListenChannel(ctx context.Context, index uint64) (*models.ChannelInfo, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		req, err := http.NewRequestWithContext(
			ctx,
			http.MethodGet,
			fmt.Sprintf(p.addr+"/client/channels/listen?index=%d", index),
			nil,
		)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Auth-Token", p.authToken)
		rsp, err := p.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		if rsp.StatusCode == http.StatusNoContent {
			rsp.Body.Close()
			continue
		}
		if rsp.StatusCode != http.StatusOK {
			rsp.Body.Close()
			return nil, fmt.Errorf("bad status code: %d (listenChannel)", rsp.StatusCode)
		}
		chanID, err := io.ReadAll(rsp.Body)
		if err != nil {
			rsp.Body.Close()
			return nil, err
		}
		rsp.Body.Close()
		return p.LoadChannel(ctx, string(chanID))
	}
}

func (p *sClient) InitChannel(ctx context.Context, channelInfo *models.ChannelInfo) error {
	reqBody, err := json.Marshal(channelInfo)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		p.addr+"/channel/init",
		bytes.NewReader(reqBody),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Auth-Token", p.authToken)
	rsp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	if rsp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status code: %d (initChannel)", rsp.StatusCode)
	}
	return err
}

func (p *sClient) LoadChannel(ctx context.Context, chanID string) (*models.ChannelInfo, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf(p.addr+"/channel/load?chanid=%s", url.QueryEscape(chanID)),
		nil,
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Auth-Token", p.authToken)
	rsp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status code: %d (loadChannel)", rsp.StatusCode)
	}
	channelInfo := &models.ChannelInfo{}
	if err := json.NewDecoder(rsp.Body).Decode(channelInfo); err != nil {
		return nil, err
	}
	if len(channelInfo.EncList) == 0 {
		return nil, errors.New("invalid enc list")
	}
	return channelInfo, nil
}

func (p *sClient) PushMessage(ctx context.Context, messageInfo *models.MessageInfo) error {
	reqBody, err := json.Marshal(messageInfo)
	if err != nil {
		panic(err)
	}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		p.addr+"/channel/chat/push",
		bytes.NewReader(reqBody),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Auth-Token", p.authToken)
	rsp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	if rsp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status code: %d (pushMessage)", rsp.StatusCode)
	}
	return err
}

func (p *sClient) LoadMessage(ctx context.Context, mhash string) (*models.MessageInfo, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf(p.addr+"/channel/chat/load?hash=%s", url.QueryEscape(mhash)),
		nil,
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Auth-Token", p.authToken)
	rsp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status code: %d (loadMessage)", rsp.StatusCode)
	}
	messageInfo := &models.MessageInfo{}
	if err := json.NewDecoder(rsp.Body).Decode(messageInfo); err != nil {
		return nil, err
	}
	return messageInfo, nil
}

func (p *sClient) CountMessages(ctx context.Context, chanID string) (uint64, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf(p.addr+"/channel/chat/size?chanid=%s", url.QueryEscape(chanID)),
		nil,
	)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Auth-Token", p.authToken)
	rsp, err := p.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("bad status code: %d (countMessages)", rsp.StatusCode)
	}
	rspBody, err := io.ReadAll(rsp.Body)
	if err != nil {
		return 0, err
	}
	size, err := strconv.ParseUint(string(rspBody), 10, 64)
	if err != nil {
		return 0, err
	}
	return size, nil
}

func (p *sClient) ListenMessage(ctx context.Context, chanID string, index uint64) (*models.MessageInfo, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		req, err := http.NewRequestWithContext(
			ctx,
			http.MethodGet,
			fmt.Sprintf(
				p.addr+"/channel/chat/listen?chanid=%s&index=%d",
				url.QueryEscape(chanID),
				index,
			),
			nil,
		)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Auth-Token", p.authToken)
		rsp, err := p.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		if rsp.StatusCode == http.StatusNoContent {
			rsp.Body.Close()
			continue
		}
		if rsp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("bad status code: %d (listenMessage)", rsp.StatusCode)
		}
		mhash, err := io.ReadAll(rsp.Body)
		if err != nil {
			rsp.Body.Close()
			return nil, err
		}
		rsp.Body.Close()
		return p.LoadMessage(ctx, string(mhash))
	}
}
