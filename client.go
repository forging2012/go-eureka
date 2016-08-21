package eureka

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"

	"github.com/st3v/go-eureka/retry"
)

type Client struct {
	endpoints     []string
	httpClient    *http.Client
	retrySelector retry.Selector
	retryLimit    retry.Allow
	retryDelay    retry.Delay
}

func NewClient(endpoints []string, options ...Option) *Client {
	for i, e := range endpoints {
		endpoints[i] = strings.TrimRight(e, " /")
	}

	c := &Client{
		endpoints:     endpoints,
		httpClient:    DefaultHTTPClient,
		retrySelector: DefaultRetrySelector,
		retryLimit:    DefaultRetryLimit,
		retryDelay:    DefaultRetryDelay,
	}

	for _, opt := range options {
		opt(c)
	}

	return c
}

func (c *Client) Register(instance *Instance) error {
	data, err := xml.Marshal(instance)
	if err != nil {
		return err
	}

	return c.retry(c.do("POST", c.appPath(instance.AppName), data, http.StatusNoContent))
}

func (c *Client) Deregister(instance *Instance) error {
	return c.retry(c.do("DELETE", c.appInstancePath(instance.AppName, instance.ID), nil, http.StatusOK))
}

func (c *Client) Heartbeat(instance *Instance) error {
	return c.retry(c.do("PUT", c.appInstancePath(instance.AppName, instance.ID), nil, http.StatusOK))
}

func (c *Client) Apps() ([]*App, error) {
	result := new(Registry)
	if err := c.retry(c.get(c.appsPath(), result)); err != nil {
		return nil, err
	}

	return result.Apps, nil
}

func (c *Client) App(appName string) (*App, error) {
	app := new(App)
	err := c.retry(c.get(c.appPath(appName), app))
	return app, err
}

func (c *Client) AppInstance(appName, instanceID string) (*Instance, error) {
	instance := new(Instance)
	err := c.retry(c.get(c.appInstancePath(appName, instanceID), instance))
	return instance, err
}

func (c *Client) Instance(instanceID string) (*Instance, error) {
	instance := new(Instance)
	err := c.retry(c.get(c.instancePath(instanceID), instance))
	return instance, err
}

func (c *Client) StatusOverride(instance *Instance, status Status) error {
	return c.retry(c.do("PUT", c.appInstanceStatusPath(instance.AppName, instance.ID, status), nil, http.StatusOK))
}

func (c *Client) RemoveStatusOverride(instance *Instance, fallback Status) error {
	return c.retry(c.do("DELETE", c.appInstanceStatusPath(instance.AppName, instance.ID, fallback), nil, http.StatusOK))
}

func (c *Client) retry(action retry.Action) error {
	return retry.NewStrategy(c.retrySelector(c.endpoints), c.retryLimit, c.retryDelay).Apply(action)
}

func (c *Client) do(method, path string, body []byte, respCode int) retry.Action {
	return func(endpoint string) error {
		req, err := http.NewRequest(method, fmt.Sprintf("%s/%s", endpoint, path), bytes.NewBuffer(body))
		if err != nil {
			return err
		}

		req.Header.Add("Content-Type", "application/xml")
		req.Header.Add("Accept", "application/xml")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return err
		}

		if resp.StatusCode != respCode {
			return fmt.Errorf("Unexpected response code %d", resp.StatusCode)
		}

		return nil
	}
}

func (c *Client) get(path string, result interface{}) retry.Action {
	return func(endpoint string) error {
		req, err := http.NewRequest("GET", fmt.Sprintf("%s/%s", endpoint, path), nil)
		if err != nil {
			return err
		}

		req.Header.Add("Accept", "application/xml")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return err
		}

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("Unexpected response code %d", resp.StatusCode)
		}

		defer resp.Body.Close()
		if err := xml.NewDecoder(resp.Body).Decode(result); err != nil {
			return err
		}

		return nil
	}
}

func (c *Client) appsPath() string {
	return "apps"
}

func (c *Client) appPath(appName string) string {
	return fmt.Sprintf("%s/%s", c.appsPath(), appName)
}

func (c *Client) appInstancePath(appName, instanceID string) string {
	return fmt.Sprintf("%s/%s", c.appPath(appName), instanceID)
}

func (c *Client) instancePath(instanceID string) string {
	return fmt.Sprintf("instances/%s", instanceID)
}

func (c *Client) appInstanceStatusPath(appName, instanceID string, status Status) string {
	return fmt.Sprintf("%s/status?value=%s", c.appInstancePath(appName, instanceID), status)
}
