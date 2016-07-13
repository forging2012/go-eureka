package fake

import (
	"encoding/xml"
	"log"
	"net/http"
	"os"

	"github.com/emicklei/go-restful"
	"github.com/st3v/go-eureka"
)

type registry struct {
	apps map[string]eureka.App
}

func NewRegistry() *registry {
	return &registry{
		apps: map[string]eureka.App{},
	}
}

func (r *registry) HttpServer(addr string, debug bool) *http.Server {
	if debug {
		restful.TraceLogger(log.New(os.Stdout, "[restful] ", log.LstdFlags|log.Lshortfile))
	}

	s := new(restful.WebService).Path("/apps").Produces(restful.MIME_XML)
	s.Route(s.POST("/{app-name}").To(r.register).Consumes(restful.MIME_XML))
	s.Route(s.DELETE("/{app-name}/{instance-id}").To(r.deregister))
	s.Route(s.PUT("/{app-name}/{instance-id}").To(r.heartbeat))
	s.Route(s.GET("/").To(r.list))
	s.Route(s.GET("/{app-name}").To(r.app))
	s.Route(s.GET("/{app-name}/{instance-id}").To(r.instance))

	return &http.Server{
		Addr:    addr,
		Handler: restful.NewContainer().Add(s),
	}
}

func (r *registry) service() *restful.WebService {
	s := new(restful.WebService)

	s.Path("/apps").Produces(restful.MIME_XML)
	s.Route(s.POST("/{app-name}").To(r.register).Consumes(restful.MIME_XML))
	s.Route(s.DELETE("/{app-name}/{instance-id}").To(r.deregister))
	s.Route(s.PUT("/{app-name}/{instance-id}").To(r.heartbeat))
	s.Route(s.GET("/").To(r.list))
	s.Route(s.GET("/{app-name}").To(r.app))
	s.Route(s.GET("/{app-name}/{instance-id}").To(r.instance))

	return s
}

func (r *registry) deregister(req *restful.Request, resp *restful.Response) {
	resp.AddHeader("Content-Type", "text/plain")

	name := req.PathParameter("app-name")
	instanceId := req.PathParameter("instance-id")

	if app, found := r.apps[name]; found {
		for i, instance := range app.Instances {
			if instance.Id == instanceId {
				app.Instances = append(app.Instances[0:i], app.Instances[i+1:]...)

				if len(app.Instances) == 0 {
					delete(r.apps, name)
				}

				resp.WriteHeader(http.StatusOK)
				return
			}
		}
	}

	resp.WriteErrorString(http.StatusNotFound, "Instance not found.")
}

func (r *registry) register(req *restful.Request, resp *restful.Response) {
	resp.AddHeader("Content-Type", "text/plain")

	name := req.PathParameter("app-name")

	instance := new(eureka.Instance)
	err := req.ReadEntity(instance)
	if err != nil {
		resp.WriteHeader(http.StatusNotAcceptable)
		return
	}

	app, found := r.apps[name]
	if !found {
		app = eureka.App{
			Name:      name,
			Instances: make([]eureka.Instance, 0, 1),
		}
	}

	for _, i := range app.Instances {
		if i.Id == instance.Id {
			resp.WriteErrorString(http.StatusMethodNotAllowed, "Instance already registered")
			return
		}
	}

	app.Instances = append(app.Instances, *instance)

	r.apps[name] = app
	resp.WriteHeader(http.StatusNoContent)
}

func (r *registry) list(req *restful.Request, resp *restful.Response) {
	apps := make([]eureka.App, 0, len(r.apps))

	for _, app := range r.apps {
		apps = append(apps, app)
	}

	payload := struct {
		XMLName xml.Name     `xml: "applications"`
		Apps    []eureka.App `xml: "application"`
	}{
		XMLName: xml.Name{Local: "applications"},
		Apps:    apps,
	}

	resp.WriteEntity(payload)
}

func (r *registry) app(req *restful.Request, resp *restful.Response) {
	name := req.PathParameter("app-name")

	app, found := r.apps[name]
	if !found {
		resp.AddHeader("Content-Type", "text/plain")
		resp.WriteErrorString(http.StatusNotFound, "App not found.")
		return
	}

	resp.WriteEntity(app)
}

func (r *registry) heartbeat(req *restful.Request, resp *restful.Response) {
	resp.AddHeader("Content-Type", "text/plain")

	name := req.PathParameter("app-name")
	instanceId := req.PathParameter("instance-id")

	if _, found := r.findInstance(name, instanceId); !found {
		resp.WriteErrorString(http.StatusNotFound, "Instance not found.")
		return
	}

	resp.WriteHeader(http.StatusOK)
}

func (r *registry) instance(req *restful.Request, resp *restful.Response) {
	name := req.PathParameter("app-name")
	instanceId := req.PathParameter("instance-id")

	if i, found := r.findInstance(name, instanceId); found {
		resp.WriteEntity(i)
		return
	}

	resp.AddHeader("Content-Type", "text/plain")
	resp.WriteErrorString(http.StatusNotFound, "Instance not found.")
}

func (r *registry) findInstance(appName, instanceId string) (*eureka.Instance, bool) {
	if app, found := r.apps[appName]; found {
		for _, i := range app.Instances {
			if i.Id == instanceId {
				return &i, true
			}
		}
	}
	return nil, false
}