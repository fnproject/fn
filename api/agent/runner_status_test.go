package agent

import (
	"context"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"sync/atomic"
	"testing"
)

// Functional behavior
// Verify returns default status if neither image nor custom health
// Verify submits a status call to the Agent (to run in container context) if image name is set
// Verify invokes custom health function if set
// Verify invokes both when custom function and image name are set
// Verify invokes custom function before status image
// Verify doesn't call status image if custom health checker function fails
// Verify order
//    1.  Gathers the container network status by looking for the network barrier (file)
//    2.  Invokes custom health func
//    3.  Submits status image; uses the network status in #1 above for the call

// Caching behavior when image name is set
// Caches the call
// Uses a previously cached call if one is in progress
// Spawns a new call if no call is in progress
//

func TestRunnerStatus_InvokesCustomFunction(t *testing.T) {

	t.Skip("Status image required for running custom function")
	customReturnStatus := map[string]string{
		"fake-status-k": "fake-status-v",
	}
	agent := new(MockAgent)
	ut := NewStatusTrackerWithAgent(agent)
	ut.customHealthCheckerFunc = func(_ context.Context) (map[string]string, error) {
		return customReturnStatus, nil
	}
	status, err := ut.Status(context.TODO(), &empty.Empty{})

	assert.Nil(t, err, "unexpected error from Status")
	assert.EqualValues(t, customReturnStatus, status.CustomStatus, "unexpected result from Status")

	agent.AssertNotCalled(t, "Submit")
}

func TestRunnerStatus_ReturnsDefaultStatus(t *testing.T) {

	agent := new(MockAgent)
	ut := NewStatusTrackerWithAgent(agent)

	// Set up some fake stats
	atomic.AddUint64(&ut.requestsReceived, 100)
	atomic.AddInt32(&ut.inflight, 2)
	atomic.AddUint64(&ut.requestsHandled, 98)

	// Get status
	status, err := ut.Status(context.TODO(), &empty.Empty{})

	assert.Nil(t, err, "unexpected error from Status")
	agent.AssertNotCalled(t, "Submit")

	// Verify retrieved
	assert.Equal(t, uint64(100), status.GetRequestsReceived(), "incorrect request received count")
	assert.Equal(t, int32(2), status.GetActive(), "incorrect active count")
	assert.Equal(t, uint64(98), status.GetRequestsHandled(), "incorrect ")

}

func TestRunnerStatus_CallsStatusImage(t *testing.T) {

	const statusImageName = "fake-image-name"
	agent := new(MockAgent)
	ut := NewStatusTrackerWithAgent(agent)

	// Setup expectations
	submitMatcher := func(c Call) bool {
		return c.Model().Image == statusImageName
	}
	agent.On("GetCall", mock.AnythingOfType("[]agent.CallOpt")).Return(error(nil)).Once()
	agent.On("Submit", mock.MatchedBy(submitMatcher)).Return(nil).Once()

	// Setup tracker and request status
	ut.imageName = statusImageName
	ut.Status(context.TODO(), &empty.Empty{})

	// Verify expectations met
	agent.AssertExpectations(t)

}

func TestRunnerStatus_InvokesCustomFuncAndCallsStatusImage(t *testing.T) {

	const statusImageName = "fake-image-name"
	agent := new(MockAgent)
	ut := NewStatusTrackerWithAgent(agent)

	// Setup expectations
	submitMatcher := func(c Call) bool {
		return c.Model().Image == statusImageName
	}
	agent.On("GetCall", mock.AnythingOfType("[]agent.CallOpt")).Return(error(nil)).Once()
	agent.On("Submit", mock.MatchedBy(submitMatcher)).Return(nil).Once()

	called := false
	// Setup tracker and request status
	ut.imageName = statusImageName
	ut.customHealthCheckerFunc = func(_ context.Context) (map[string]string, error) {
		called = true
		return nil, nil
	}
	ut.Status(context.TODO(), &empty.Empty{})

	// Verify expectations met
	agent.AssertExpectations(t)
	assert.True(t, called, "customHealthChecker not invoked as expected")
}

func TestRunnerStatus_NoStatusImageCallWhenCheckerFails(t *testing.T) {

	const statusImageName = "fake-image-name"
	agent := new(MockAgent)
	ut := NewStatusTrackerWithAgent(agent)

	called := false
	// Setup tracker and request status
	ut.imageName = statusImageName
	ut.customHealthCheckerFunc = func(_ context.Context) (map[string]string, error) {
		called = true
		return nil, errors.New("custom checker failed")
	}
	ut.Status(context.TODO(), &empty.Empty{})

	// Verify expectations met
	assert.True(t, called, "customHealthChecker not invoked as expected")
	agent.AssertNotCalled(t, "GetCall")
	agent.AssertNotCalled(t, "Submit")
}
