package twitter

import (
	"net/http"

	"github.com/dghubble/sling"
)

// AccountService provides a method for account credential verification.
type AccountService struct {
	sling *sling.Sling
}

// newAccountService returns a new AccountService.
func newAccountService(sling *sling.Sling) *AccountService {
	return &AccountService{
		sling: sling.Path("account/"),
	}
}

// AccountVerifyParams are the params for AccountService.VerifyCredentials.
type AccountVerifyParams struct {
	IncludeEntities *bool `url:"include_entities,omitempty"`
	SkipStatus      *bool `url:"skip_status,omitempty"`
	IncludeEmail    *bool `url:"include_email,omitempty"`
}

// VerifyCredentials returns the authorized user if credentials are valid and
// returns an error otherwise.
// Requires a user auth context.
// https://dev.twitter.com/rest/reference/get/account/verify_credentials
func (s *AccountService) VerifyCredentials(params *AccountVerifyParams) (*User, *http.Response, error) {
	user := new(User)
	apiError := new(APIError)
	resp, err := s.sling.New().Get("verify_credentials.json").QueryStruct(params).Receive(user, apiError)
	return user, resp, relevantError(err, *apiError)
}
