package remote

import (
	"context"
	"fmt"
	"io"

	"github.com/pkg/errors"
	"github.com/tychoish/birch/mrpc/mongowire"
	"github.com/tychoish/birch/mrpc/shell"
	"github.com/tychoish/jasper/options"
	"github.com/tychoish/jasper/scripting"
)

const (
	ScriptingCreateCommand    = "create_scripting"
	ScriptingGetCommand       = "get_scripting"
	ScriptingSetupCommand     = "setup_scripting"
	ScriptingCleanupCommand   = "cleanup_scripting"
	ScriptingRunCommand       = "run_scripting"
	ScriptingRunScriptCommand = "run_script_scripting"
	ScriptingBuildCommand     = "build_scripting"
	ScriptingTestCommand      = "test_scripting"
)

func (s *mdbService) scriptingGet(ctx context.Context, w io.Writer, msg mongowire.Message) {
	req := &scriptingGetRequest{}
	if !s.serviceScriptingRequest(ctx, w, msg, req, ScriptingGetCommand) {
		return
	}

	harness := s.getHarness(ctx, w, req.ID, ScriptingGetCommand)
	if harness == nil {
		return
	}

	s.serviceScriptingResponse(ctx, w, nil, ScriptingGetCommand)
}

func (s *mdbService) scriptingCreate(ctx context.Context, w io.Writer, msg mongowire.Message) {
	req := &scriptingCreateRequest{}
	if !s.serviceScriptingRequest(ctx, w, msg, req, ScriptingCreateCommand) {
		return
	}

	opts, err := options.NewScriptingHarness(req.Params.Type)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("problem creating harness options: %w", err), ScriptingCreateCommand)
		return
	}

	if err = s.unmarshaler(req.Params.Options, opts); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("problem unmarshalling options: %w", err), ScriptingCreateCommand)
		return
	}

	harness, err := s.harnessCache.Create(s.manager, opts)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("problem creating harness: %w", err), ScriptingCreateCommand)
		return
	}

	s.serviceScriptingResponse(ctx, w, makeScriptingCreateResponse(harness.ID()), ScriptingCreateCommand)
}

func (s *mdbService) scriptingSetup(ctx context.Context, w io.Writer, msg mongowire.Message) {
	req := &scriptingSetupRequest{}
	if !s.serviceScriptingRequest(ctx, w, msg, req, ScriptingSetupCommand) {
		return
	}

	harness := s.getHarness(ctx, w, req.ID, ScriptingSetupCommand)
	if harness == nil {
		return
	}
	if err := harness.Setup(ctx); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("problem setting up harness: %w", err), ScriptingSetupCommand)
		return
	}

	s.serviceScriptingResponse(ctx, w, nil, ScriptingSetupCommand)
}

func (s *mdbService) scriptingCleanup(ctx context.Context, w io.Writer, msg mongowire.Message) {
	req := &scriptingCleanupRequest{}
	if !s.serviceScriptingRequest(ctx, w, msg, req, ScriptingCleanupCommand) {
		return
	}

	harness := s.getHarness(ctx, w, req.ID, ScriptingCleanupCommand)
	if harness == nil {
		return
	}
	if err := harness.Cleanup(ctx); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("problem cleaning up harness: %w", err), ScriptingCleanupCommand)
		return
	}

	s.serviceScriptingResponse(ctx, w, nil, ScriptingCleanupCommand)
}

func (s *mdbService) scriptingRun(ctx context.Context, w io.Writer, msg mongowire.Message) {
	req := &scriptingRunRequest{}
	if !s.serviceScriptingRequest(ctx, w, msg, req, ScriptingRunCommand) {
		return
	}

	harness := s.getHarness(ctx, w, req.Params.ID, ScriptingRunCommand)
	if harness == nil {
		return
	}
	if err := harness.Run(ctx, req.Params.Args); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("problem running command: %w", err), ScriptingRunCommand)
		return
	}

	s.serviceScriptingResponse(ctx, w, nil, ScriptingRunCommand)
}

func (s *mdbService) scriptingRunScript(ctx context.Context, w io.Writer, msg mongowire.Message) {
	req := &scriptingRunScriptRequest{}
	if !s.serviceScriptingRequest(ctx, w, msg, req, ScriptingRunScriptCommand) {
		return
	}

	harness := s.getHarness(ctx, w, req.Params.ID, ScriptingRunScriptCommand)
	if harness == nil {
		return
	}
	if err := harness.RunScript(ctx, req.Params.Script); err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("problem running script: %w", err), ScriptingRunScriptCommand)
		return
	}

	s.serviceScriptingResponse(ctx, w, nil, ScriptingRunScriptCommand)
}

func (s *mdbService) scriptingBuild(ctx context.Context, w io.Writer, msg mongowire.Message) {
	req := &scriptingBuildRequest{}
	if !s.serviceScriptingRequest(ctx, w, msg, req, ScriptingBuildCommand) {
		return
	}

	harness := s.getHarness(ctx, w, req.Params.ID, ScriptingBuildCommand)
	if harness == nil {
		return
	}
	path, err := harness.Build(ctx, req.Params.Dir, req.Params.Args)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("problem building artifact: %w", err), ScriptingBuildCommand)
		return
	}

	s.serviceScriptingResponse(ctx, w, makeScriptingBuildResponse(path), ScriptingBuildCommand)
}

func (s *mdbService) scriptingTest(ctx context.Context, w io.Writer, msg mongowire.Message) {
	req := &scriptingTestRequest{}
	if !s.serviceScriptingRequest(ctx, w, msg, req, ScriptingTestCommand) {
		return
	}

	harness := s.getHarness(ctx, w, req.Params.ID, ScriptingTestCommand)
	if harness == nil {
		return
	}
	results, err := harness.Test(ctx, req.Params.Dir, req.Params.Options...)
	if err != nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("problem running tests: %w", err), ScriptingTestCommand)
		return
	}

	s.serviceScriptingResponse(ctx, w, makeScriptingTestResponse(results), ScriptingTestCommand)
}

func (s *mdbService) serviceScriptingRequest(ctx context.Context, w io.Writer, msg mongowire.Message, req interface{}, command string) bool {
	if s.harnessCache == nil {
		shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.New("scripting environment is not supported"), command)
		return false
	}

	if req != nil {
		doc, err := shell.RequestMessageToDocument(msg)
		if err != nil {
			shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not read request: %w", err), command)
			return false
		}
		if err := s.readPayload(doc, req); err != nil {
			shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not parse request: %w", err), command)
			return false
		}
	}

	return true
}

func (s *mdbService) getHarness(ctx context.Context, w io.Writer, id, command string) scripting.Harness {
	harness, err := s.harnessCache.Get(id)
	if err == nil {
		return harness
	}

	shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, errors.Wrapf(err, "problem fetching scripting harness with id %s", id), command)
	return nil
}

func (s *mdbService) serviceScriptingResponse(ctx context.Context, w io.Writer, resp interface{}, command string) {
	if resp != nil {
		payload, err := s.makePayload(resp)
		if err != nil {
			shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not parse response: %w", err), command)
			return
		}

		shellResp, err := shell.ResponseToMessage(mongowire.OP_REPLY, payload)
		if err != nil {
			shell.WriteErrorResponse(ctx, w, mongowire.OP_REPLY, fmt.Errorf("could not make response: %w", err), command)
			return
		}

		shell.WriteResponse(ctx, w, shellResp, command)
	} else {
		shell.WriteOKResponse(ctx, w, mongowire.OP_REPLY, command)
	}
}
