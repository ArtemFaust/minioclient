package customoperations

import (
	"context"
	"minioclient/global"
	"net/http"
	"net/url"
	"strings"
)

// Custom HealthCheck
func HealthCheck(client *global.GlobalClient, ctx context.Context) (int, error) {
	url := client.MinioClient.EndpointURL()
	resp, e := do(ctx, url, http.MethodHead)
	if e != nil {
		return 0, e
	}
	return resp.StatusCode, nil

}

// Метод специфичный для CEPH RGW - получение статистики бакета
func GetRgwBucketStats(client *global.GlobalClient, bucketname string, objectkey string) (map[string]string, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rgwbucketstats := map[string]string{}

	server, e := GetServerInfo(client, ctx)
	if e != nil || !strings.Contains(server, "Ceph Object Gateway") {
		return rgwbucketstats, e
	}

	params := url.Values{}
	params.Add("read-stats", "")

	url, e := client.MakePresignUrlForBucketOperation(ctx, bucketname, objectkey, params)
	if e != nil {
		return rgwbucketstats, e
	}

	resp, e := do(ctx, url, http.MethodHead)
	if e != nil {
		return rgwbucketstats, e
	}
	defer resp.Body.Close()

	rgwbucketstats["objectCount"] = resp.Header.Get("X-Rgw-Object-Count")
	rgwbucketstats["bytesUsed"] = resp.Header.Get("X-Rgw-Bytes-Used")
	rgwbucketstats["maxBuckets"] = resp.Header.Get("X-Rgw-Quota-Max-Buckets")

	return rgwbucketstats, nil
}

// Метод получения информации о версии сервера
func GetServerInfo(client *global.GlobalClient, ctx context.Context) (string, error) {
	url := client.MinioClient.EndpointURL()
	resp, e := do(ctx, url, http.MethodHead)
	if e != nil {
		return "UNDEFINED", e
	}
	if resp.Header.Get("Server") != "" {
		return resp.Header.Get("Server"), nil
	}
	return "UNDEFINED", e
}

// Общий метод выполнения запроса
func do(ctx context.Context, url *url.URL, method string) (*http.Response, error) {
	var resp *http.Response
	req, e := http.NewRequestWithContext(ctx, method, url.String(), nil)
	if e != nil {
		return resp, e
	}

	resp, e = http.DefaultClient.Do(req)
	if e != nil {
		return resp, e
	}

	return resp, nil
}
