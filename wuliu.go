// Docs: https://market.aliyun.com/products/57126001/cmapi021863.html
package alicloudapislim

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"time"
)

type WuliuClient struct {
	AppCode string

	providers []WuliuProvider
}

type WuliuProvider struct {
	Code string
	Name string
}

type WuliuStatus struct {
	Code         string
	Number       string
	Status       string
	CompanyName  string
	CompanyLogo  string
	CompanyPhone string
	CourierName  string
	CourierPhone string
	UpdatedAt    time.Time
	TimeElapsed  string
	Items        []WuliuStatusItem
}

type WuliuStatusItem struct {
	Desc string
	Time time.Time
}

func NewWuliuClient(appCode string) *WuliuClient {
	return &WuliuClient{
		AppCode: appCode,
	}
}

func (client WuliuClient) request(ctx context.Context, path string, target interface{}) error {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://wuliu.market.alicloudapi.com"+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "APPCODE "+client.AppCode)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(target)
}

func (client *WuliuClient) MustGetProviders(ctx context.Context) []WuliuProvider {
	providers, err := client.GetProviders(ctx)
	if err != nil {
		panic(err)
	}
	return providers
}

func (client *WuliuClient) GetProviders(ctx context.Context) ([]WuliuProvider, error) {
	if len(client.providers) > 0 {
		return client.providers, nil
	}
	var ret struct {
		Status  string            `json:"status"`
		Message string            `json:"msg"`
		Result  map[string]string `json:"result"`
	}
	if err := client.request(ctx, "/getExpressList", &ret); err != nil {
		return nil, err
	}
	if ret.Status != "200" {
		return nil, fmt.Errorf("failed to get wuliu providers: status %s, message %s returned", ret.Status, ret.Message)
	}
	var providers []WuliuProvider
	for code, name := range ret.Result {
		providers = append(providers, WuliuProvider{
			Code: code,
			Name: name,
		})
	}
	if len(providers) > 0 {
		sort.Slice(providers, func(i, j int) bool { return providers[i].Code < providers[j].Code })
		client.providers = providers
	}
	return providers, nil
}

func (client WuliuClient) MustGetProvidersForNumber(ctx context.Context, no string) []WuliuProvider {
	providers, err := client.GetProvidersForNumber(ctx, no)
	if err != nil {
		panic(err)
	}
	return providers
}

func (client WuliuClient) GetProvidersForNumber(ctx context.Context, no string) ([]WuliuProvider, error) {
	values := url.Values{}
	values.Set("no", no)
	var ret struct {
		Status  string `json:"status"`
		Message string `json:"msg"`
		Number  string `json:"number"`
		List    []struct {
			Code string `json:"type"`
			Name string `json:"name"`
		} `json:"list"`
	}
	if err := client.request(ctx, "/exCompany?"+values.Encode(), &ret); err != nil {
		return nil, err
	}
	if ret.Status != "0" {
		return nil, fmt.Errorf("failed to get wuliu provider: status %s, message %s returned", ret.Status, ret.Message)
	}
	var providers []WuliuProvider
	for _, item := range ret.List {
		providers = append(providers, WuliuProvider{
			Code: item.Code,
			Name: item.Name,
		})
	}
	return providers, nil
}

func (client WuliuClient) MustGetStatusForNumber(ctx context.Context, code, no string) *WuliuStatus {
	status, err := client.GetStatusForNumber(ctx, code, no)
	if err != nil {
		panic(err)
	}
	return status
}

func (client WuliuClient) GetStatusForNumber(ctx context.Context, code, no string) (*WuliuStatus, error) {
	values := url.Values{}
	values.Set("type", code)
	values.Set("no", no)
	var ret struct {
		Status  string `json:"status"`
		Message string `json:"msg"`
		Result  struct {
			Number string `json:"number"`
			Type   string `json:"type"`
			List   []struct {
				Time   string `json:"time"`
				Status string `json:"status"`
			} `json:"list"`
			DeliveryStatus string `json:"deliverystatus"`
			IsSign         string `json:"issign"`
			ExpName        string `json:"expName"`
			ExpSite        string `json:"expSite"`
			ExpPhone       string `json:"expPhone"`
			Courier        string `json:"courier"`
			CourierPhone   string `json:"courierPhone"`
			UpdateTime     string `json:"updateTime"`
			TakeTime       string `json:"takeTime"`
			Logo           string `json:"logo"`
		} `json:"result"`
	}
	if err := client.request(ctx, "/kdi?"+values.Encode(), &ret); err != nil {
		return nil, err
	}
	if ret.Status != "0" {
		return nil, fmt.Errorf("failed to get wuliu status: status %s, message %s returned", ret.Status, ret.Message)
	}
	status := ret.Result.DeliveryStatus
	switch status {
	case "0":
		status = "快递收件(揽件)"
	case "1":
		status = "在途中"
	case "2":
		status = "正在派件"
	case "3":
		status = "已签收"
	case "4":
		status = "派送失败"
	case "5":
		status = "疑难件"
	case "6":
		status = "退件签收"
	}
	loc := time.FixedZone("UTC+8", 8*60*60)
	updatedAt, _ := time.ParseInLocation("2006-01-02 15:04:05", ret.Result.UpdateTime, loc)
	items := []WuliuStatusItem{}
	for _, item := range ret.Result.List {
		time, _ := time.ParseInLocation("2006-01-02 15:04:05", item.Time, loc)
		items = append(items, WuliuStatusItem{
			Desc: item.Status,
			Time: time,
		})
	}
	return &WuliuStatus{
		Code:         ret.Result.Type,
		Number:       ret.Result.Number,
		Status:       status,
		CompanyName:  ret.Result.ExpName,
		CompanyLogo:  ret.Result.Logo,
		CompanyPhone: ret.Result.ExpPhone,
		CourierName:  ret.Result.Courier,
		CourierPhone: ret.Result.CourierPhone,
		UpdatedAt:    updatedAt,
		TimeElapsed:  ret.Result.TakeTime,
		Items:        items,
	}, nil
}
