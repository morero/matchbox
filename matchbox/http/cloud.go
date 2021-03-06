package http

import (
	"bytes"
	"net/http"
	"strings"
	"time"

	"context"
	"github.com/Sirupsen/logrus"
	cloudinit "github.com/coreos/coreos-cloudinit/config"

	"github.com/coreos/matchbox/matchbox/server"
	pb "github.com/coreos/matchbox/matchbox/server/serverpb"
)

// CloudConfig defines a cloud-init config.
type CloudConfig struct {
	Content string
}

// cloudHandler returns a handler that responds with the cloud config matching
// the request.
func (s *Server) cloudHandler(core server.Server) ContextHandler {
	fn := func(ctx context.Context, w http.ResponseWriter, req *http.Request) {
		group, err := groupFromContext(ctx)
		if err != nil {
			s.logger.WithFields(logrus.Fields{
				"labels": labelsFromRequest(nil, req),
			}).Infof("No matching group")
			http.NotFound(w, req)
			return
		}

		profile, err := core.ProfileGet(ctx, &pb.ProfileGetRequest{Id: group.Profile})
		if err != nil {
			s.logger.WithFields(logrus.Fields{
				"labels":     labelsFromRequest(nil, req),
				"group":      group.Id,
				"group_name": group.Name,
			}).Infof("No profile named: %s", group.Profile)
			http.NotFound(w, req)
			return
		}

		contents, err := core.CloudGet(ctx, profile.CloudId)
		if err != nil {
			s.logger.WithFields(logrus.Fields{
				"labels":     labelsFromRequest(nil, req),
				"group":      group.Id,
				"group_name": group.Name,
				"profile":    group.Profile,
			}).Infof("No cloud-config template named: %s", profile.CloudId)
			http.NotFound(w, req)
			return
		}

		// match was successful
		s.logger.WithFields(logrus.Fields{
			"labels":  labelsFromRequest(nil, req),
			"group":   group.Id,
			"profile": profile.Id,
		}).Debug("Matched a cloud-config template")

		// collect data for rendering
		data, err := collectVariables(req, group)
		if err != nil {
			s.logger.Errorf("error collecting variables: %v", err)
			http.NotFound(w, req)
			return
		}

		// render the template of a cloud config with data
		var buf bytes.Buffer
		err = s.renderTemplate(&buf, data, contents)
		if err != nil {
			http.NotFound(w, req)
			return
		}

		config := buf.String()
		if !cloudinit.IsCloudConfig(config) && !cloudinit.IsScript(config) {
			s.logger.Error("error parsing user-data")
			http.NotFound(w, req)
			return
		}

		if cloudinit.IsCloudConfig(config) {
			if _, err = cloudinit.NewCloudConfig(config); err != nil {
				s.logger.Errorf("error parsing cloud config: %v", err)
				http.NotFound(w, req)
				return
			}
		}
		http.ServeContent(w, req, "", time.Time{}, strings.NewReader(config))
	}
	return ContextHandlerFunc(fn)
}
