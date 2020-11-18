package remote

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"syscall"
	"time"

	"github.com/cdr/gimlet"
	"github.com/cdr/grip/message"
	"github.com/deciduosity/jasper"
	"github.com/deciduosity/jasper/options"
	"github.com/deciduosity/jasper/scripting"
	"github.com/pkg/errors"
)

// Service defines a REST service that provides a remote manager, using
// gimlet to publish routes.
type Service struct {
	hostID    string
	manager   jasper.Manager
	harnesses scripting.HarnessCache
}

// NewManagerService creates a service object around an existing
// manager. You must access the application and routes via the App()
// method separately. The constructor wraps basic managers with a
// manager implementation that does locking.
func NewRestService(m jasper.Manager) *Service {
	return &Service{
		manager:   m,
		harnesses: scripting.NewCache(),
	}
}

// App constructs and returns a gimlet application for this
// service. It attaches no middleware and does not start the service.
func (s *Service) App(ctx context.Context) *gimlet.APIApp {
	s.hostID, _ = os.Hostname()

	app := gimlet.NewApp()

	app.AddRoute("/").Version(1).Get().Handler(s.rootRoute)
	app.AddRoute("/id").Version(1).Get().Handler(s.id)
	app.AddRoute("/create").Version(1).Post().Handler(s.createProcess)
	app.AddRoute("/download").Version(1).Post().Handler(s.downloadFile)
	app.AddRoute("/list/{filter}").Version(1).Get().Handler(s.listProcesses)
	app.AddRoute("/list/group/{name}").Version(1).Get().Handler(s.listGroupMembers)
	app.AddRoute("/process/{id}").Version(1).Get().Handler(s.getProcess)
	app.AddRoute("/process/{id}/tags").Version(1).Get().Handler(s.getProcessTags)
	app.AddRoute("/process/{id}/tags").Version(1).Delete().Handler(s.deleteProcessTags)
	app.AddRoute("/process/{id}/tags").Version(1).Post().Handler(s.addProcessTag)
	app.AddRoute("/process/{id}/wait").Version(1).Get().Handler(s.waitForProcess)
	app.AddRoute("/process/{id}/respawn").Version(1).Get().Handler(s.respawnProcess)
	app.AddRoute("/process/{id}/metrics").Version(1).Get().Handler(s.processMetrics)
	app.AddRoute("/process/{id}/logs/{count}").Version(1).Get().Handler(s.getLogStream)
	app.AddRoute("/process/{id}/signal/{signal}").Version(1).Patch().Handler(s.signalProcess)
	app.AddRoute("/process/{id}/trigger/signal/{trigger-id}").Version(1).Patch().Handler(s.registerSignalTriggerID)
	app.AddRoute("/signal/event/{name}").Version(1).Patch().Handler(s.signalEvent)
	app.AddRoute("/scripting/create/{type}").Version(1).Post().Handler(s.scriptingCreate)
	app.AddRoute("/scripting/{id}").Version(1).Get().Handler(s.scriptingCheck)
	app.AddRoute("/scripting/{id}").Version(1).Delete().Handler(s.scriptingCleanup)
	app.AddRoute("/scripting/{id}/setup").Version(1).Post().Handler(s.scriptingSetup)
	app.AddRoute("/scripting/{id}/run").Version(1).Post().Handler(s.scriptingRun)
	app.AddRoute("/scripting/{id}/script").Version(1).Post().Handler(s.scriptingRunScript)
	app.AddRoute("/scripting/{id}/build").Version(1).Post().Handler(s.scriptingBuild)
	app.AddRoute("/scripting/{id}/test").Version(1).Post().Handler(s.scriptingTest)
	app.AddRoute("/logging/id/{id}").Version(1).Post().Handler(s.loggingCacheCreate)
	app.AddRoute("/logging/id/{id}").Version(1).Get().Handler(s.loggingCacheGet)
	app.AddRoute("/logging/id/{id}").Version(1).Delete().Handler(s.loggingCacheDelete)
	app.AddRoute("/logging/id/{id}/close").Version(1).Delete().Handler(s.loggingCacheCloseAndRemove)
	app.AddRoute("/logging/id/{id}/send").Version(1).Post().Handler(s.loggingSendMessages)
	app.AddRoute("/logging/clear").Version(1).Delete().Handler(s.loggingCacheClear)
	app.AddRoute("/logging/prune/{time}").Version(1).Delete().Handler(s.loggingCachePrune)
	app.AddRoute("/file/write").Version(1).Put().Handler(s.writeFile)
	app.AddRoute("/clear").Version(1).Post().Handler(s.clearManager)
	app.AddRoute("/close").Version(1).Delete().Handler(s.closeManager)

	return app
}

func getProcInfoNoHang(ctx context.Context, p jasper.Process) jasper.ProcessInfo {
	ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	return p.Info(ctx)
}

func writeError(rw http.ResponseWriter, err gimlet.ErrorResponse) {
	gimlet.WriteJSONResponse(rw, err.StatusCode, err)
}

func (s *Service) rootRoute(rw http.ResponseWriter, r *http.Request) {
	gimlet.WriteJSON(rw, struct {
		HostID string `json:"host_id"`
		Active bool   `json:"active"`
	}{
		HostID: s.hostID,
		Active: true,
	})
}

func (s *Service) id(rw http.ResponseWriter, r *http.Request) {
	gimlet.WriteJSON(rw, s.manager.ID())
}

func (s *Service) createProcess(rw http.ResponseWriter, r *http.Request) {
	opts := &options.Create{}
	if err := gimlet.GetJSON(r.Body, opts); err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Message:    errors.Wrap(err, "problem reading request").Error(),
		})
		return
	}
	ctx := r.Context()

	if err := opts.Validate(); err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Message:    errors.Wrap(err, "invalid creation options").Error(),
		})
		return
	}

	pctx, cancel := context.WithCancel(context.Background())

	proc, err := s.manager.CreateProcess(pctx, opts)
	if err != nil {
		cancel()
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Message:    errors.Wrap(err, "problem submitting request").Error(),
		})
		return
	}

	if err := proc.RegisterTrigger(ctx, func(_ jasper.ProcessInfo) {
		cancel()
	}); err != nil {
		info := getProcInfoNoHang(ctx, proc)
		cancel()
		// If we get an error registering a trigger, then we should make sure
		// that the reason for it isn't just because the process has exited
		// already, since that should not be considered an error.
		if !info.Complete {
			writeError(rw, gimlet.ErrorResponse{
				StatusCode: http.StatusInternalServerError,
				Message:    errors.Wrap(err, "problem registering trigger").Error(),
			})
			return
		}
	}

	gimlet.WriteJSON(rw, getProcInfoNoHang(ctx, proc))
}

func (s *Service) listProcesses(rw http.ResponseWriter, r *http.Request) {
	filter := options.Filter(gimlet.GetVars(r)["filter"])
	if err := filter.Validate(); err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Message:    errors.Wrap(err, "invalid input").Error(),
		})
		return
	}

	ctx := r.Context()

	procs, err := s.manager.List(ctx, filter)
	if err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    err.Error(),
		})
		return
	}

	out := []jasper.ProcessInfo{}
	for _, proc := range procs {
		out = append(out, getProcInfoNoHang(ctx, proc))
	}

	gimlet.WriteJSON(rw, out)
}

func (s *Service) listGroupMembers(rw http.ResponseWriter, r *http.Request) {
	name := gimlet.GetVars(r)["name"]

	ctx := r.Context()

	procs, err := s.manager.Group(ctx, name)
	if err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    err.Error(),
		})
		return
	}

	out := []jasper.ProcessInfo{}
	for _, proc := range procs {
		out = append(out, getProcInfoNoHang(ctx, proc))
	}

	gimlet.WriteJSON(rw, out)
}

func (s *Service) getProcess(rw http.ResponseWriter, r *http.Request) {
	id := gimlet.GetVars(r)["id"]
	ctx := r.Context()
	proc, err := s.manager.Get(ctx, id)
	if err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    errors.Wrapf(err, "no process '%s' found", id).Error(),
		})
		return
	}

	info := getProcInfoNoHang(ctx, proc)
	gimlet.WriteJSON(rw, info)
}

func (s *Service) processMetrics(rw http.ResponseWriter, r *http.Request) {
	id := gimlet.GetVars(r)["id"]
	ctx := r.Context()
	proc, err := s.manager.Get(ctx, id)
	if err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    errors.Wrapf(err, "no process '%s' found", id).Error(),
		})
		return
	}

	info := getProcInfoNoHang(ctx, proc)
	gimlet.WriteJSON(rw, message.CollectProcessInfoWithChildren(int32(info.PID)))
}

func (s *Service) getProcessTags(rw http.ResponseWriter, r *http.Request) {
	id := gimlet.GetVars(r)["id"]
	ctx := r.Context()
	proc, err := s.manager.Get(ctx, id)
	if err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    errors.Wrapf(err, "no process '%s' found", id).Error(),
		})
		return
	}

	gimlet.WriteJSON(rw, proc.GetTags())
}

func (s *Service) deleteProcessTags(rw http.ResponseWriter, r *http.Request) {
	id := gimlet.GetVars(r)["id"]
	ctx := r.Context()
	proc, err := s.manager.Get(ctx, id)
	if err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    errors.Wrapf(err, "no process '%s' found", id).Error(),
		})
		return
	}

	proc.ResetTags()
	gimlet.WriteJSON(rw, struct{}{})
}

func (s *Service) addProcessTag(rw http.ResponseWriter, r *http.Request) {
	id := gimlet.GetVars(r)["id"]
	ctx := r.Context()
	proc, err := s.manager.Get(ctx, id)
	if err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    errors.Wrapf(err, "no process '%s' found", id).Error(),
		})
		return
	}

	newtags := r.URL.Query()["add"]
	if len(newtags) == 0 {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Message:    "no new tags specified",
		})
		return
	}

	for _, t := range newtags {
		proc.Tag(t)
	}

	gimlet.WriteJSON(rw, struct{}{})
}

func (s *Service) waitForProcess(rw http.ResponseWriter, r *http.Request) {
	id := gimlet.GetVars(r)["id"]
	ctx := r.Context()
	proc, err := s.manager.Get(ctx, id)
	if err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    errors.Wrapf(err, "no process '%s' found", id).Error(),
		})
		return
	}

	exitCode, err := proc.Wait(ctx)
	if err != nil && exitCode == -1 {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Message:    err.Error(),
		})
		return
	}

	gimlet.WriteJSON(rw, exitCode)
}

func (s *Service) respawnProcess(rw http.ResponseWriter, r *http.Request) {
	id := gimlet.GetVars(r)["id"]
	ctx := r.Context()

	proc, err := s.manager.Get(r.Context(), id)
	if err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    errors.Wrapf(err, "no process '%s' found", id).Error(),
		})
		return
	}

	// Spawn a new context so that the process' context is not potentially
	// canceled by the request's. See how createProcess() does this same thing.
	pctx, cancel := context.WithCancel(context.Background())
	newProc, err := proc.Respawn(pctx)
	if err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Message:    err.Error(),
		})
		cancel()
		return
	}
	if err := s.manager.Register(ctx, newProc); err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message: errors.Wrap(
				err, "failed to register respawned process").Error(),
		})
		cancel()
		return
	}

	if err := newProc.RegisterTrigger(ctx, func(_ jasper.ProcessInfo) {
		cancel()
	}); err != nil {
		newProcInfo := getProcInfoNoHang(ctx, newProc)
		cancel()
		if !newProcInfo.Complete {
			writeError(rw, gimlet.ErrorResponse{
				StatusCode: http.StatusInternalServerError,
				Message: errors.Wrap(
					err, "failed to register trigger on respawned process").Error(),
			})
			return
		}
	}

	info := getProcInfoNoHang(ctx, newProc)
	gimlet.WriteJSON(rw, info)
}

func (s *Service) signalProcess(rw http.ResponseWriter, r *http.Request) {
	vars := gimlet.GetVars(r)
	id := vars["id"]
	sig, err := strconv.Atoi(vars["signal"])
	if err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Message:    errors.Wrapf(err, "problem converting signal '%s'", vars["signal"]).Error(),
		})
		return
	}

	ctx := r.Context()
	proc, err := s.manager.Get(ctx, id)
	if err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    errors.Wrapf(err, "no process '%s' found", id).Error(),
		})
		return
	}

	if err := proc.Signal(ctx, syscall.Signal(sig)); err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Message:    err.Error(),
		})
		return
	}

	gimlet.WriteJSON(rw, struct{}{})
}

func (s *Service) downloadFile(rw http.ResponseWriter, r *http.Request) {
	var opts options.Download
	if err := gimlet.GetJSON(r.Body, &opts); err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Message:    errors.Wrap(err, "problem reading request").Error(),
		})
		return
	}

	if err := opts.Validate(); err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Message:    errors.Wrap(err, "problem validating download options").Error(),
		})
		return
	}

	if err := opts.Download(r.Context()); err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrapf(err, "problem occurred during file download for URL %s", opts.URL).Error(),
		})
		return
	}

	gimlet.WriteJSON(rw, struct{}{})
}

func (s *Service) getLogStream(rw http.ResponseWriter, r *http.Request) {
	vars := gimlet.GetVars(r)
	id := vars["id"]
	count, err := strconv.Atoi(vars["count"])
	if err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Message:    errors.Wrapf(err, "problem converting count '%s'", vars["count"]).Error(),
		})
		return
	}

	ctx := r.Context()

	proc, err := s.manager.Get(ctx, id)
	if err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    errors.Wrapf(err, "no process '%s' found", id).Error(),
		})
		return
	}

	stream := jasper.LogStream{}
	stream.Logs, err = jasper.GetInMemoryLogStream(ctx, proc, count)

	if err == io.EOF {
		stream.Done = true
	} else if err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrapf(err, "could not get logs for process '%s'", id).Error(),
		})
		return
	}

	gimlet.WriteJSON(rw, stream)
}

func (s *Service) signalEvent(rw http.ResponseWriter, r *http.Request) {
	vars := gimlet.GetVars(r)
	name := vars["name"]
	ctx := r.Context()

	if err := jasper.SignalEvent(ctx, name); err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrapf(err, "problem signaling event named '%s'", name).Error(),
		})
		return
	}

	gimlet.WriteJSON(rw, struct{}{})
}

func (s *Service) writeFile(rw http.ResponseWriter, r *http.Request) {
	var opts options.WriteFile
	if err := gimlet.GetJSON(r.Body, &opts); err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Message:    errors.Wrap(err, "problem reading request").Error(),
		})
		return
	}

	if err := opts.Validate(); err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Message:    errors.Wrap(err, "problem validating file write options").Error(),
		})
		return
	}

	if err := opts.DoWrite(); err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrapf(err, "problem occurred during file write to %s", opts.Path).Error(),
		})
		return
	}

	if err := opts.SetPerm(); err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrapf(err, "problem occurred while setting permissions on file %s", opts.Path).Error(),
		})
		return
	}

	gimlet.WriteJSON(rw, struct{}{})
}

func (s *Service) clearManager(rw http.ResponseWriter, r *http.Request) {
	s.manager.Clear(r.Context())
	gimlet.WriteJSON(rw, struct{}{})
}

func (s *Service) closeManager(rw http.ResponseWriter, r *http.Request) {
	if err := s.manager.Close(r.Context()); err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Message:    err.Error(),
		})
		return
	}

	gimlet.WriteJSON(rw, struct{}{})
}

func (s *Service) registerSignalTriggerID(rw http.ResponseWriter, r *http.Request) {
	vars := gimlet.GetVars(r)
	id := vars["id"]
	triggerID := vars["trigger-id"]
	ctx := r.Context()

	proc, err := s.manager.Get(ctx, id)
	if err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Message:    errors.Wrapf(err, "no process '%s' found", id).Error(),
		})
		return
	}

	sigTriggerID := jasper.SignalTriggerID(triggerID)
	makeTrigger, ok := jasper.GetSignalTriggerFactory(sigTriggerID)
	if !ok {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Message:    errors.Errorf("could not find signal trigger with id '%s'", sigTriggerID).Error(),
		})
		return
	}

	if err := proc.RegisterSignalTrigger(ctx, makeTrigger()); err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    errors.Wrapf(err, "problem registering signal trigger with id '%s'", sigTriggerID).Error(),
		})
		return
	}

	gimlet.WriteJSON(rw, struct{}{})
}

type restLoggingCacheSize struct {
	Size int `json:"size"`
}

func (s *Service) loggingCacheSize(rw http.ResponseWriter, r *http.Request) {
	gimlet.WriteJSON(rw, &restLoggingCacheSize{Size: s.manager.LoggingCache(r.Context()).Len()})
}

func (s *Service) loggingCacheCreate(rw http.ResponseWriter, r *http.Request) {
	args := &options.Output{}
	id := gimlet.GetVars(r)["id"]
	if err := gimlet.GetJSON(r.Body, args); err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Message:    errors.Wrap(err, "problem parsing options").Error(),
		})
		return
	}

	cl, err := s.manager.LoggingCache(r.Context()).Create(id, args)
	if err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Message:    errors.Wrap(err, "problem creating loggers").Error(),
		})
		return
	}

	gimlet.WriteJSON(rw, cl)
}

func (s *Service) loggingCacheDelete(rw http.ResponseWriter, r *http.Request) {
	s.manager.LoggingCache(r.Context()).Remove(gimlet.GetVars(r)["id"])
}

func (s *Service) loggingCachePrune(rw http.ResponseWriter, r *http.Request) {
	ts, err := time.Parse(time.RFC3339, gimlet.GetVars(r)["time"])
	if err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Message:    errors.Wrapf(err, "problem parsing timestamp").Error(),
		})
		return
	}

	s.manager.LoggingCache(r.Context()).Prune(ts)
}

func (s *Service) loggingCacheGet(rw http.ResponseWriter, r *http.Request) {
	gimlet.WriteJSON(rw, s.manager.LoggingCache(r.Context()).Get(gimlet.GetVars(r)["id"]))
}

func (s *Service) loggingSendMessages(rw http.ResponseWriter, r *http.Request) {
	id := gimlet.GetVars(r)["id"]
	cache := s.manager.LoggingCache(r.Context())
	logger := cache.Get(id)
	if logger == nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    fmt.Sprintf("logger '%s' does not exist", id),
		})
		return
	}

	payload := &options.LoggingPayload{}
	if err := gimlet.GetJSON(r.Body, payload); err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    errors.Wrapf(err, "problem parsing payload for %s", id).Error(),
		})
		return
	}

	if err := logger.Send(payload); err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Message:    err.Error(),
		})
		return
	}

	gimlet.WriteJSON(rw, struct{}{})
}

func (s *Service) loggingCacheCloseAndRemove(rw http.ResponseWriter, r *http.Request) {
	id := gimlet.GetVars(r)["id"]
	lc := s.manager.LoggingCache(r.Context())
	if lc == nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    "logging cache is not supported",
		})
		return
	}

	if err := lc.CloseAndRemove(r.Context(), id); err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    err.Error(),
		})
		return
	}

	gimlet.WriteJSON(rw, struct{}{})
}

func (s *Service) loggingCacheClear(rw http.ResponseWriter, r *http.Request) {
	lc := s.manager.LoggingCache(r.Context())
	if lc == nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    "logging cache is not supported",
		})
		return
	}

	if err := lc.Clear(r.Context()); err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    err.Error(),
		})
		return
	}

	gimlet.WriteJSON(rw, struct{}{})
}

func (s *Service) scriptingCreate(rw http.ResponseWriter, r *http.Request) {
	seopt, err := options.NewScriptingHarness(gimlet.GetVars(r)["type"])
	if err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Message:    err.Error(),
		})
		return
	}

	if err = gimlet.GetJSON(r.Body, seopt); err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    err.Error(),
		})
		return
	}

	se, err := s.harnesses.Create(s.manager, seopt)
	if err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Message:    err.Error(),
		})
		return
	}

	gimlet.WriteJSON(rw, struct {
		ID string `json:"id"`
	}{
		ID: se.ID(),
	})
}

func (s *Service) scriptingCheck(rw http.ResponseWriter, r *http.Request) {
	_, err := s.harnesses.Get(gimlet.GetVars(r)["id"])
	if err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    err.Error(),
		})
		return
	}

	gimlet.WriteJSON(rw, struct{}{})
}

func (s *Service) scriptingSetup(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	se, err := s.harnesses.Get(gimlet.GetVars(r)["id"])
	if err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    err.Error(),
		})
		return
	}

	if err := se.Setup(ctx); err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    err.Error(),
		})
		return
	}

	gimlet.WriteJSON(rw, struct{}{})
}

func (s *Service) scriptingRun(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	se, err := s.harnesses.Get(gimlet.GetVars(r)["id"])
	if err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    err.Error(),
		})
		return
	}

	args := &struct {
		Args []string `json:"args"`
	}{}
	if err := gimlet.GetJSON(r.Body, args); err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    err.Error(),
		})
		return
	}

	if err := se.Run(ctx, args.Args); err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Message:    err.Error(),
		})
		return
	}

	gimlet.WriteJSON(rw, struct{}{})
}

func (s *Service) scriptingRunScript(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	se, err := s.harnesses.Get(gimlet.GetVars(r)["id"])
	if err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    err.Error(),
		})
		return
	}

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    err.Error(),
		})
		return
	}

	if err := se.RunScript(ctx, string(data)); err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Message:    err.Error(),
		})
		return
	}

	gimlet.WriteJSON(rw, struct{}{})
}

func (s *Service) scriptingBuild(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	se, err := s.harnesses.Get(gimlet.GetVars(r)["id"])
	if err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    err.Error(),
		})
		return
	}

	args := &struct {
		Directory string   `json:"directory"`
		Args      []string `json:"args"`
	}{}
	if err = gimlet.GetJSON(r.Body, args); err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    err.Error(),
		})
		return
	}
	path, err := se.Build(ctx, args.Directory, args.Args)
	if err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Message:    err.Error(),
		})
		return
	}

	gimlet.WriteJSON(rw, struct {
		Path string `json:"path"`
	}{
		Path: path,
	})
}

func (s *Service) scriptingTest(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	se, err := s.harnesses.Get(gimlet.GetVars(r)["id"])
	if err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    err.Error(),
		})
		return
	}

	args := &struct {
		Directory string                  `json:"directory"`
		Options   []scripting.TestOptions `json:"options"`
	}{}
	if err = gimlet.GetJSON(r.Body, args); err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    err.Error(),
		})
		return
	}
	var errOut string
	res, err := se.Test(ctx, args.Directory, args.Options...)
	if err != nil {
		errOut = err.Error()
	}

	gimlet.WriteJSON(rw, struct {
		Results []scripting.TestResult `json:"results"`
		Error   string                 `json:"error"`
	}{
		Results: res,
		Error:   errOut,
	})
}

func (s *Service) scriptingCleanup(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	se, err := s.harnesses.Get(gimlet.GetVars(r)["id"])
	if err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    err.Error(),
		})
		return
	}

	if err := se.Cleanup(ctx); err != nil {
		writeError(rw, gimlet.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Message:    err.Error(),
		})
		return
	}
	gimlet.WriteJSON(rw, struct{}{})
}
