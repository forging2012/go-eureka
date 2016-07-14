package eureka

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

func init() {
	rand.Seed(time.Now().Unix())
}

type Option func(*Client)

type Client struct {
	endpoints  []string
	httpClient *http.Client
}

func HttpClient(c *http.Client) Option {
	return func(client *Client) {
		client.httpClient = c
	}
}

func NewClient(endpoints []string, options ...Option) *Client {
	c := &Client{
		endpoints:  endpoints,
		httpClient: http.DefaultClient,
	}

	for _, opt := range options {
		opt(c)
	}

	return c
}

func (c *Client) Register(instance Instance) error {
	data, err := xml.Marshal(instance)
	if err != nil {
		return err
	}

	return c.do("POST", c.appURI(instance.AppName), data, http.StatusNoContent)
}

func (c *Client) Deregister(instance Instance) error {
	return c.do("DELETE", c.appInstanceURI(instance.AppName, instance.Id), nil, http.StatusOK)
}

func (c *Client) Heartbeat(instance Instance) error {
	return c.do("PUT", c.appInstanceURI(instance.AppName, instance.Id), nil, http.StatusOK)
}

func (c *Client) Apps() ([]App, error) {
	result := new(Registry)
	if err := c.get(c.appsURI(), result); err != nil {
		return nil, err
	}

	return result.Apps, nil
}

func (c *Client) App(appName string) (App, error) {
	app := App{}
	err := c.get(c.appURI(appName), &app)
	return app, err
}

func (c *Client) AppInstance(appName, instanceId string) (Instance, error) {
	instance := Instance{}
	err := c.get(c.appInstanceURI(appName, instanceId), &instance)
	return instance, err
}

func (c *Client) Instance(instanceId string) (Instance, error) {
	instance := Instance{}
	err := c.get(c.instanceURI(instanceId), &instance)
	return instance, err
}

func (c *Client) do(method, uri string, body []byte, respCode int) error {
	req, err := http.NewRequest(method, uri, bytes.NewBuffer(body))
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

func (c *Client) get(uri string, result interface{}) error {
	req, err := http.NewRequest("GET", uri, nil)
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

func (c *Client) endpoint() string {
	return strings.TrimRight(c.endpoints[rand.Intn(len(c.endpoints))], " /")
}

func (c *Client) appsURI() string {
	return fmt.Sprintf("%s/apps", c.endpoint())
}

func (c *Client) appURI(appName string) string {
	return fmt.Sprintf("%s/%s", c.appsURI(), appName)
}

func (c *Client) appInstanceURI(appName, instanceId string) string {
	return fmt.Sprintf("%s/%s", c.appURI(appName), instanceId)
}

func (c *Client) instanceURI(instanceId string) string {
	return fmt.Sprintf("%s/instances/%s", c.endpoint(), instanceId)
}
