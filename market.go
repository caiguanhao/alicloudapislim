package alicloudapislim

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type MarketClient struct {
	accessKeyId     string
	accessKeySecret string
}

type MarketProduct struct {
	Id        string
	Name      string
	Remaining int
	Used      int
	Unit      string
}

type MarketProductDetails struct {
	Id          string
	Name        string
	Description string
	Options     []MarketProductOption
}

type MarketProductOption struct {
	Code string
	Name string
}

type MarketProductOptionWithPrice struct {
	Id       string
	Code     string
	Duration int
	Cycle    string
	Price    string
}

func NewMarketClient(accessKeyId, accessKeySecret string) *MarketClient {
	return &MarketClient{
		accessKeyId:     accessKeyId,
		accessKeySecret: accessKeySecret,
	}
}

func (client MarketClient) GetProducts(ctx context.Context) ([]MarketProduct, error) {
	return client.getProducts(ctx, 1)
}

func (client MarketClient) getProducts(ctx context.Context, pageNum int) ([]MarketProduct, error) {
	params := url.Values{}
	params.Set("Action", "DescribeApiMetering")
	params.Set("type", "1")
	params.Set("pageNum", strconv.Itoa(pageNum))
	var resp struct {
		PageSize   int    `json:"PageSize"`
		Message    string `json:"Message"`
		PageNumber int    `json:"PageNumber"`
		Version    string `json:"Version"`
		Count      int    `json:"Count"`
		Fatal      bool   `json:"Fatal"`
		Code       string `json:"Code"`
		Success    bool   `json:"Success"`
		Result     []struct {
			ProductName string `json:"ProductName"`
			AliyunPk    int64  `json:"AliyunPk"`
			ProductCode string `json:"ProductCode"`
			TotalQuota  int    `json:"TotalQuota"`
			TotalUsage  int    `json:"TotalUsage"`
			Unit        string `json:"Unit"`
		} `json:"Result"`
	}
	err := client.request(ctx, params, &resp)
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("failed to get metering info: code %s, message %s returned", resp.Code, resp.Message)
	}
	products := []MarketProduct{}
	for _, item := range resp.Result {
		products = append(products, MarketProduct{
			Id:        item.ProductCode,
			Name:      item.ProductName,
			Remaining: item.TotalQuota,
			Used:      item.TotalUsage,
			Unit:      item.Unit,
		})
	}
	totalPages := int(resp.Count / resp.PageSize)
	for i := 2; i <= totalPages; i++ {
		prods, err := client.getProducts(ctx, i)
		if err != nil {
			return nil, err
		}
		products = append(products, prods...)
	}
	return products, nil
}

func (client MarketClient) GetProduct(ctx context.Context, id string) (*MarketProductDetails, error) {
	params := url.Values{}
	params.Set("Action", "DescribeProduct")
	params.Set("Code", id)
	var resp struct {
		ProductSkus struct {
			ProductSku []struct {
				ChargeType string `json:"ChargeType"`
				Modules    struct {
					Module []struct {
						Properties struct {
							Property []struct {
								PropertyValues struct {
									PropertyValue []struct {
										Type        string `json:"Type"`
										DisplayName string `json:"DisplayName"`
										Value       string `json:"Value"`
									} `json:"PropertyValue"`
								} `json:"PropertyValues"`
								Key string `json:"Key"`
							} `json:"Property"`
						} `json:"Properties"`
						Code string `json:"Code"`
					} `json:"Module"`
				} `json:"Modules"`
			} `json:"ProductSku"`
		} `json:"ProductSkus"`
		Code             string `json:"Code"`
		ShortDescription string `json:"ShortDescription"`
		Name             string `json:"Name"`
		Type             string `json:"Type"`
	}
	err := client.request(ctx, params, &resp)
	if err != nil {
		return nil, err
	}
	options := []MarketProductOption{}
	for _, sku := range resp.ProductSkus.ProductSku {
		for _, module := range sku.Modules.Module {
			if module.Code == "package_version" {
				for _, option := range module.Properties.Property {
					if option.Key == "package_version" {
						for _, value := range option.PropertyValues.PropertyValue {
							options = append(options, MarketProductOption{
								Code: value.Value,
								Name: value.DisplayName,
							})
						}
					}
				}
			}
		}
	}
	return &MarketProductDetails{
		Id:          resp.Code,
		Name:        resp.Name,
		Description: resp.ShortDescription,
		Options:     options,
	}, err
}

func (client MarketClient) GetPrice(ctx context.Context, id, option string) (*MarketProductOptionWithPrice, error) {
	params := url.Values{}
	params.Set("Action", "DescribePrice")
	params.Set("OrderType", "INSTANCE_BUY")
	commodity, _ := json.Marshal(struct {
		Components  map[string]string `json:"components"`
		ProductCode string            `json:"productCode"`
	}{
		map[string]string{"package_version": option},
		id,
	})
	params.Set("Commodity", string(commodity))
	var resp struct {
		ProductCode   string  `json:"ProductCode"`
		TradePrice    float64 `json:"TradePrice"`
		OriginalPrice float64 `json:"OriginalPrice"`
		DiscountPrice float64 `json:"DiscountPrice"`
		Currency      string  `json:"Currency"`
		Duration      int     `json:"Duration"`
		Cycle         string  `json:"Cycle"`
	}
	err := client.request(ctx, params, &resp)
	if err != nil {
		return nil, err
	}
	return &MarketProductOptionWithPrice{
		Id:       id,
		Code:     option,
		Duration: resp.Duration,
		Cycle:    resp.Cycle,
		Price:    fmt.Sprintf("%.2f", resp.TradePrice),
	}, err
}

func (client MarketClient) CreateOrder(ctx context.Context, option MarketProductOptionWithPrice, overrides ...interface{}) (string, error) {
	params := url.Values{}
	params.Set("Action", "CreateOrder")
	params.Set("ClientToken", randomString(64))
	params.Set("OrderType", "INSTANCE_BUY") // INSTANCE_BUY, INSTANCE_RENEW or INSTANCE_UPGRADE
	params.Set("PaymentType", "AUTO")       // AUTO or HAND
	commodity, _ := json.Marshal(struct {
		Components   map[string]string `json:"components"`
		SkuCode      string            `json:"skuCode"`
		Duration     int               `json:"duration"`
		PricingCycle string            `json:"pricingCycle"`
		ProductCode  string            `json:"productCode"`
	}{
		map[string]string{"package_version": option.Code},
		"prepay",
		option.Duration,
		option.Cycle,
		option.Id,
	})
	params.Set("Commodity", string(commodity))
	for i := 0; i < len(overrides)/2; i++ {
		if a, ok := overrides[2*i].(string); ok {
			if b, ok := overrides[2*i+1].(string); ok {
				params.Set(a, b)
			}
		}
	}
	fmt.Println(params)
	var resp struct {
		OrderId string `json:"OrderId"`
	}
	err := client.request(ctx, params, &resp)
	if err != nil {
		return "", err
	}
	return resp.OrderId, nil
}

func (client MarketClient) request(ctx context.Context, params url.Values, target interface{}) error {
	ts := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	params.Set("Format", "json")
	params.Set("Version", "2015-11-01")
	params.Set("AccessKeyId", client.accessKeyId)
	params.Set("SignatureMethod", "HMAC-SHA1")
	params.Set("Timestamp", ts)
	params.Set("SignatureVersion", "1.0")
	params.Set("SignatureNonce", randomString(64))
	query := buildQueryString(params)
	signature := sign(client.accessKeySecret, urlEncode(query))
	params.Set("Signature", signature)
	req, err := http.NewRequestWithContext(ctx, "GET", "https://market.aliyuncs.com/?"+params.Encode(), nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		var err struct {
			Code    string `json:"Code"`
			Message string `json:"Message"`
		}
		json.NewDecoder(resp.Body).Decode(&err)
		return fmt.Errorf("server responded status %d with code %s and message %s returned", resp.StatusCode, err.Code, err.Message)
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func sign(secret string, query string) string {
	mac := hmac.New(sha1.New, []byte(secret+"&"))
	mac.Write([]byte("GET&%2F&" + query))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func urlEncode(input string) string {
	return strings.Replace(url.QueryEscape(input), "+", "%20", -1)
}

func buildQueryString(params url.Values) string {
	keys := make([]string, 0, len(params))
	for key := range params {
		if key == "Signature" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	queries := make([]string, 0, len(params))
	for _, key := range keys {
		query := fmt.Sprintf("%s=%s", urlEncode(key), urlEncode(params.Get(key)))
		queries = append(queries, query)
	}
	queryString := strings.Join(queries, "&")
	return queryString
}

func randomString(n int) string {
	const alphanum = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	var bytes = make([]byte, n)
	rand.Read(bytes)
	for i, b := range bytes {
		bytes[i] = alphanum[b%byte(len(alphanum))]
	}
	return string(bytes)
}
