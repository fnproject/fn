package tests

import (
	"github.com/fnproject/fn_go/client/apps"
	"github.com/fnproject/fn_go/models"
	"reflect"
	"strings"
	"testing"
)

func TestAppDeleteNotFound(t *testing.T) {
	t.Parallel()
	s := SetupHarness()

	cfg := &apps.DeleteAppsAppParams{
		App:     "missing-app",
		Context: s.Context,
	}

	_, err := s.Client.Apps.DeleteAppsApp(cfg)

	if _, ok := err.(*apps.DeleteAppsAppNotFound); !ok {
		t.Errorf("Error during app delete: we should get HTTP 404, but got: %s", err.Error())
	}
}

func TestAppGetNotFound(t *testing.T) {
	t.Parallel()
	s := SetupHarness()
	defer s.Cleanup()

	cfg := &apps.GetAppsAppParams{
		App:     "missing-app",
		Context: s.Context,
	}
	_, err := s.Client.Apps.GetAppsApp(cfg)

	if _, ok := err.(*apps.GetAppsAppNotFound); !ok {
		t.Errorf("Error during get: we should get HTTP 404, but got: %s", err.Error())
	}

	if !strings.Contains(err.(*apps.GetAppsAppNotFound).Payload.Error.Message, "App not found") {
		t.Errorf("Error during app delete: unexpeted error `%s`, wanted `App not found`", err.Error())
	}
}

func TestAppCreateNoConfigSuccess(t *testing.T) {
	t.Parallel()
	s := SetupHarness()
	defer s.Cleanup()

	resp, err := s.PostApp(&models.App{
		Name: s.AppName,
	})

	if err != nil {
		t.Errorf("Failed to create simple app %v", err)
		return
	}

	if resp.Payload.App.Name != s.AppName {
		t.Errorf("app name in response %s does not match new app %s ", resp.Payload.App.Name, s.AppName)
	}

}

func TestSetAppMetadataOnCreate(t *testing.T) {
	t.Parallel()
	for _, tci := range createMetadataValidCases {
		// iterator mutation meets parallelism... pfft
		tc := tci
		t.Run("valid_"+tc.name, func(t *testing.T) {
			t.Parallel()
			s := SetupHarness()
			defer s.Cleanup()

			app, err := s.PostApp(&models.App{
				Name:     s.AppName,
				Metadata: tc.metadata,
			})

			if err != nil {
				t.Fatalf("Failed to create app with valid metadata %v got error %v", tc.metadata, err)
			}

			gotMd := app.Payload.App.Metadata
			if !MetadataEquivalent(gotMd, tc.metadata) {
				t.Errorf("Returned metadata %v does not match set metadata %v", gotMd, tc.metadata)
			}

			getApp := s.AppMustExist(t, s.AppName)

			if !MetadataEquivalent(getApp.Metadata, tc.metadata) {
				t.Errorf("GET metadata '%v' does not match set metadata %v", getApp.Metadata, tc.metadata)
			}

		})
	}

	for _, tci := range createMetadataErrorCases {
		// iterator mutation meets parallelism... pfft
		tc := tci
		t.Run("invalid_"+tc.name, func(ti *testing.T) {
			ti.Parallel()
			s := SetupHarness()
			defer s.Cleanup()

			_, err := s.PostApp(&models.App{
				Name:     s.AppName,
				Metadata: tc.metadata,
			})

			if err == nil {
				t.Fatalf("Created app with invalid metadata %v but expected error", tc.metadata)
			}

			if _, ok := err.(*apps.PostAppsBadRequest); !ok {
				t.Errorf("Expecting bad request for invalid metadata, got %v", err)
			}

		})
	}
}

func TestUpdateAppMetadataOnPatch(t *testing.T) {
	t.Parallel()

	for _, tci := range updateMetadataValidCases {
		// iterator mutation meets parallelism... pfft
		tc := tci
		t.Run("valid_"+tc.name, func(t *testing.T) {
			t.Parallel()
			s := SetupHarness()
			defer s.Cleanup()

			s.GivenAppExists(t, &models.App{
				Name:     s.AppName,
				Metadata: tc.initialMetadata,
			})

			res, err := s.Client.Apps.PatchAppsApp(&apps.PatchAppsAppParams{
				App:     s.AppName,
				Context: s.Context,
				Body: &models.AppWrapper{
					App: &models.App{
						Metadata: tc.change,
					},
				},
			})

			if err != nil {
				t.Fatalf("Failed to patch metadata with %v on app: %v", tc.change, err)
			}

			gotMd := res.Payload.App.Metadata
			if !MetadataEquivalent(gotMd, tc.expected) {
				t.Errorf("Returned metadata %v does not match set metadata %v", gotMd, tc.expected)
			}

			getApp := s.AppMustExist(t, s.AppName)

			if !MetadataEquivalent(getApp.Metadata, tc.expected) {
				t.Errorf("GET metadata '%v' does not match set metadata %v", getApp.Metadata, tc.expected)
			}
		})
	}

	for _, tci := range updateMetadataErrorCases {
		// iterator mutation meets parallelism... pfft
		tc := tci
		t.Run("invalid_"+tc.name, func(t *testing.T) {
			t.Parallel()
			s := SetupHarness()
			defer s.Cleanup()

			s.GivenAppExists(t, &models.App{
				Name:     s.AppName,
				Metadata: tc.initialMetadata,
			})

			_, err := s.Client.Apps.PatchAppsApp(&apps.PatchAppsAppParams{
				App:     s.AppName,
				Context: s.Context,
				Body: &models.AppWrapper{
					App: &models.App{
						Metadata: tc.change,
					},
				},
			})

			if err == nil {
				t.Errorf("patched app with invalid metadata %v but expected error", tc.change)
			}
			if _, ok := err.(*apps.PatchAppsAppBadRequest); !ok {
				t.Errorf("Expecting bad request for invalid metadata, got %v", err)
			}

		})
	}
}

func TestAppCreateWithConfigSuccess(t *testing.T) {
	t.Parallel()
	s := SetupHarness()
	defer s.Cleanup()

	validConfig := map[string]string{"A": "a"}
	appPayload, err := s.PostApp(&models.App{
		Name:   s.AppName,
		Config: validConfig,
	})

	if err != nil {
		t.Fatalf("Failed to create app with valid config got %v", err)
	}

	if !reflect.DeepEqual(validConfig, appPayload.Payload.App.Config) {
		t.Errorf("Expecting config %v but got %v in response", validConfig, appPayload.Payload.App.Config)
	}

}

func TestAppInsect(t *testing.T) {
	t.Parallel()
	s := SetupHarness()
	defer s.Cleanup()

	validConfig := map[string]string{"A": "a"}

	s.GivenAppExists(t, &models.App{Name: s.AppName,
		Config: validConfig})

	appOk, err := s.Client.Apps.GetAppsApp(&apps.GetAppsAppParams{
		App:     s.AppName,
		Context: s.Context,
	})

	if err != nil {
		t.Fatalf("Expected valid response to get app, got %v", err)
	}

	if !reflect.DeepEqual(validConfig, appOk.Payload.App.Config) {
		t.Errorf("Returned config %v does not match requested config %v", appOk.Payload.App.Config, validConfig)
	}
}

func TestAppPatchConfig(t *testing.T) {
	t.Parallel()

	for _, tci := range updateConfigCases {
		// iterator mutation meets parallelism... pfft
		tc := tci
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := SetupHarness()
			defer s.Cleanup()

			s.GivenAppExists(t, &models.App{
				Name:   s.AppName,
				Config: tc.intialConfig,
			})

			patch, err := s.Client.Apps.PatchAppsApp(&apps.PatchAppsAppParams{
				App: s.AppName,
				Body: &models.AppWrapper{
					App: &models.App{
						Config: tc.change,
					},
				},
				Context: s.Context,
			})

			if err != nil {
				t.Fatalf("Failed to patch app with valid value %v, %v", tc.change, err)
			}

			if !ConfigEquivalent(patch.Payload.App.Config, tc.expected) {
				t.Errorf("Expected returned app config to be %v, but was %v", tc.expected, patch.Payload.App.Config)
			}

		})
	}

}

func TestAppDuplicate(t *testing.T) {
	t.Parallel()
	s := SetupHarness()
	defer s.Cleanup()

	s.GivenAppExists(t, &models.App{Name: s.AppName})

	_, err := s.PostApp(&models.App{Name: s.AppName})

	if _, ok := err.(*apps.PostAppsConflict); !ok {
		t.Errorf("Expecting conflict response on duplicate app, got %v", err)
	}

}
