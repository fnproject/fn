package manager

import (
	"encoding/base64"
	"encoding/pem"
	"io/ioutil"
	"os"
	"testing"

	"github.com/docker/swarmkit/ca"
	cautils "github.com/docker/swarmkit/ca/testutils"
	"github.com/docker/swarmkit/manager/state/raft"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

// Tests updating a kek on a raftDEK object.
func TestRaftDEKUpdateKEK(t *testing.T) {
	startData := RaftDEKData{
		EncryptionKeys: raft.EncryptionKeys{CurrentDEK: []byte("first dek")},
	}
	startKEK := ca.KEKData{}

	// because UpdateKEK returns a PEMKeyHeaders interface, we need to cast to check
	// values
	updateDEKAndCast := func(dekdata RaftDEKData, oldKEK ca.KEKData, newKEK ca.KEKData) RaftDEKData {
		result := dekdata.UpdateKEK(oldKEK, newKEK)
		raftDekObj, ok := result.(RaftDEKData)
		require.True(t, ok)
		return raftDekObj
	}

	// nothing changes if we are updating a kek and they're both nil
	result := updateDEKAndCast(startData, startKEK, ca.KEKData{Version: 2})
	require.Equal(t, result, startData)

	// when moving from unlocked to locked, a "needs rotation" header is generated but no
	// pending header is generated
	updatedKEK := ca.KEKData{KEK: []byte("something"), Version: 1}
	result = updateDEKAndCast(startData, startKEK, updatedKEK)
	require.NotEqual(t, startData, result)
	require.True(t, result.NeedsRotation)
	require.Equal(t, startData.CurrentDEK, result.CurrentDEK)
	require.Nil(t, result.PendingDEK)

	// this is whether or not pending exists
	startData.PendingDEK = []byte("pending")
	result = updateDEKAndCast(startData, startKEK, updatedKEK)
	require.NotEqual(t, startData, result)
	require.True(t, result.NeedsRotation)
	require.Equal(t, startData.CurrentDEK, result.CurrentDEK)
	require.Equal(t, startData.PendingDEK, result.PendingDEK)

	// if we are going from locked to unlocked, nothing happens
	result = updateDEKAndCast(startData, updatedKEK, startKEK)
	require.Equal(t, startData, result)
	require.False(t, result.NeedsRotation)

	// if we are going to locked to another locked, nothing happens
	result = updateDEKAndCast(startData, updatedKEK, ca.KEKData{KEK: []byte("other"), Version: 4})
	require.Equal(t, startData, result)
	require.False(t, result.NeedsRotation)
}

func TestRaftDEKMarshalUnmarshal(t *testing.T) {
	startData := RaftDEKData{
		EncryptionKeys: raft.EncryptionKeys{CurrentDEK: []byte("first dek")},
	}
	kek := ca.KEKData{}

	headers, err := startData.MarshalHeaders(kek)
	require.NoError(t, err)
	require.Len(t, headers, 1)

	// can't unmarshal with the wrong kek
	_, err = RaftDEKData{}.UnmarshalHeaders(headers, ca.KEKData{KEK: []byte("something")})
	require.Error(t, err)

	// we can unmarshal what was marshalled with the right kek
	toData, err := RaftDEKData{}.UnmarshalHeaders(headers, kek)
	require.NoError(t, err)
	require.Equal(t, startData, toData)

	// try the other headers as well
	startData.PendingDEK = []byte("Hello")
	headers, err = startData.MarshalHeaders(kek)
	require.NoError(t, err)
	require.Len(t, headers, 2)

	// we can unmarshal what was marshalled
	toData, err = RaftDEKData{}.UnmarshalHeaders(headers, kek)
	require.NoError(t, err)
	require.Equal(t, startData, toData)

	// try the other headers as well
	startData.NeedsRotation = true
	startData.PendingDEK = nil
	headers, err = startData.MarshalHeaders(kek)
	require.NoError(t, err)
	require.Len(t, headers, 2)

	// we can unmarshal what was marshalled
	toData, err = RaftDEKData{}.UnmarshalHeaders(headers, kek)
	require.NoError(t, err)
	require.Equal(t, startData, toData)

	// If there is a pending header, but no current header, set will fail
	headers = map[string]string{
		pemHeaderRaftPendingDEK: headers[pemHeaderRaftDEK],
	}
	_, err = RaftDEKData{}.UnmarshalHeaders(headers, kek)
	require.Error(t, err)
	require.Contains(t, err.Error(), "pending DEK, but no current DEK")
}

// NewRaftDEKManager creates a key if one doesn't exist
func TestNewRaftDEKManager(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "manager-new-dek-manager-")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	paths := ca.NewConfigPaths(tempDir)
	cert, key, err := cautils.CreateRootCertAndKey("cn")
	require.NoError(t, err)

	krw := ca.NewKeyReadWriter(paths.Node, nil, nil)
	require.NoError(t, krw.Write(cert, key, nil))

	keyBytes, err := ioutil.ReadFile(paths.Node.Key)
	require.NoError(t, err)
	require.NotContains(t, string(keyBytes), pemHeaderRaftDEK) // headers are not written

	dekManager, err := NewRaftDEKManager(krw) // this should create a new DEK and write it to the file
	require.NoError(t, err)

	keyBytes, err = ioutil.ReadFile(paths.Node.Key)
	require.NoError(t, err)
	require.Contains(t, string(keyBytes), pemHeaderRaftDEK) // header is written now

	keys := dekManager.GetKeys()
	require.NotNil(t, keys.CurrentDEK)
	require.Nil(t, keys.PendingDEK)
	require.False(t, dekManager.NeedsRotation())

	// If one exists, nothing is updated
	dekManager, err = NewRaftDEKManager(krw) // this should create a new DEK and write it to the file
	require.NoError(t, err)

	keyBytes2, err := ioutil.ReadFile(paths.Node.Key)
	require.NoError(t, err)
	require.Equal(t, keyBytes, keyBytes2)

	require.Equal(t, keys, dekManager.GetKeys())
	require.False(t, dekManager.NeedsRotation())
}

// NeedsRotate returns true if there is a PendingDEK or a NeedsRotation flag
func TestRaftDEKManagerNeedsRotateGetKeys(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "manager-maybe-get-data-")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	paths := ca.NewConfigPaths(tempDir)

	// if there is no PendingDEK, and no NeedsRotation flag:  NeedsRotation=false
	keys := raft.EncryptionKeys{CurrentDEK: []byte("hello")}
	dekManager, err := NewRaftDEKManager(
		ca.NewKeyReadWriter(paths.Node, nil, RaftDEKData{EncryptionKeys: keys}))
	require.NoError(t, err)

	require.False(t, dekManager.NeedsRotation())
	require.Equal(t, keys, dekManager.GetKeys())

	// if there is a PendingDEK, and no NeedsRotation flag:  NeedsRotation=true
	keys = raft.EncryptionKeys{CurrentDEK: []byte("hello"), PendingDEK: []byte("another")}
	dekManager, err = NewRaftDEKManager(
		ca.NewKeyReadWriter(paths.Node, nil, RaftDEKData{EncryptionKeys: keys}))
	require.NoError(t, err)

	require.True(t, dekManager.NeedsRotation())
	require.Equal(t, keys, dekManager.GetKeys())

	// if there is a PendingDEK, and a NeedsRotation flag:  NeedsRotation=true
	keys = raft.EncryptionKeys{CurrentDEK: []byte("hello"), PendingDEK: []byte("another")}
	dekManager, err = NewRaftDEKManager(
		ca.NewKeyReadWriter(paths.Node, nil, RaftDEKData{
			EncryptionKeys: keys,
			NeedsRotation:  true,
		}))
	require.NoError(t, err)

	require.True(t, dekManager.NeedsRotation())
	require.Equal(t, keys, dekManager.GetKeys())

	// if there no PendingDEK, and a NeedsRotation flag:  NeedsRotation=true and
	// GetKeys attempts to create a pending key and write it to disk.  However, writing
	// will error (because there is no key on disk atm), and then the original keys will
	// be returned.
	keys = raft.EncryptionKeys{CurrentDEK: []byte("hello")}
	krw := ca.NewKeyReadWriter(paths.Node, nil, RaftDEKData{
		EncryptionKeys: keys,
		NeedsRotation:  true,
	})
	dekManager, err = NewRaftDEKManager(krw)
	require.NoError(t, err)

	require.True(t, dekManager.NeedsRotation())
	require.Equal(t, keys, dekManager.GetKeys())
	h, _ := krw.GetCurrentState()
	dekData, ok := h.(RaftDEKData)
	require.True(t, ok)
	require.True(t, dekData.NeedsRotation)

	// if there no PendingDEK, and a NeedsRotation flag:  NeedsRotation=true and
	// GetKeys attempts to create a pending key and write it to disk.  If successful,
	// it returns the new keys
	keys = raft.EncryptionKeys{CurrentDEK: []byte("hello")}
	krw = ca.NewKeyReadWriter(paths.Node, nil, RaftDEKData{
		EncryptionKeys: keys,
		NeedsRotation:  true,
	})
	dekManager, err = NewRaftDEKManager(krw)

	require.NoError(t, err)
	cert, key, err := cautils.CreateRootCertAndKey("cn")
	require.NoError(t, err)
	require.NoError(t, krw.Write(cert, key, nil))

	require.True(t, dekManager.NeedsRotation())
	updatedKeys := dekManager.GetKeys()
	require.Equal(t, keys.CurrentDEK, updatedKeys.CurrentDEK)
	require.NotNil(t, updatedKeys.PendingDEK)
	require.True(t, dekManager.NeedsRotation())

	h, _ = krw.GetCurrentState()
	dekData, ok = h.(RaftDEKData)
	require.True(t, ok)
	require.False(t, dekData.NeedsRotation)
}

func TestRaftDEKManagerUpdateKeys(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "manager-update-keys-")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	paths := ca.NewConfigPaths(tempDir)
	cert, key, err := cautils.CreateRootCertAndKey("cn")
	require.NoError(t, err)

	keys := raft.EncryptionKeys{
		CurrentDEK: []byte("key1"),
		PendingDEK: []byte("key2"),
	}
	krw := ca.NewKeyReadWriter(paths.Node, nil, RaftDEKData{
		EncryptionKeys: keys,
		NeedsRotation:  true,
	})
	require.NoError(t, krw.Write(cert, key, nil))

	dekManager, err := NewRaftDEKManager(krw)
	require.NoError(t, err)

	newKeys := raft.EncryptionKeys{
		CurrentDEK: []byte("new current"),
	}
	require.NoError(t, dekManager.UpdateKeys(newKeys))
	// don't run GetKeys, because NeedsRotation is true and it'd just generate a new one

	h, _ := krw.GetCurrentState()
	dekData, ok := h.(RaftDEKData)
	require.True(t, ok)
	require.True(t, dekData.NeedsRotation)

	// UpdateKeys so there is no CurrentDEK: all the headers should be wiped out
	require.NoError(t, dekManager.UpdateKeys(raft.EncryptionKeys{}))
	require.Equal(t, raft.EncryptionKeys{}, dekManager.GetKeys())
	require.False(t, dekManager.NeedsRotation())

	h, _ = krw.GetCurrentState()
	require.Nil(t, h)

	keyBytes, err := ioutil.ReadFile(paths.Node.Key)
	require.NoError(t, err)
	keyBlock, _ := pem.Decode(keyBytes)
	require.NotNil(t, keyBlock)

	// the only header remaining should be the kek version
	require.Len(t, keyBlock.Headers, 1)
	require.Contains(t, keyBlock.Headers, "kek-version")
}

func TestRaftDEKManagerMaybeUpdateKEK(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "manager-maybe-update-kek-")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	paths := ca.NewConfigPaths(tempDir)
	cert, key, err := cautils.CreateRootCertAndKey("cn")
	require.NoError(t, err)

	keys := raft.EncryptionKeys{CurrentDEK: []byte("current dek")}

	// trying to update a KEK will error if the version is the same but the kek is different
	krw := ca.NewKeyReadWriter(paths.Node, nil, RaftDEKData{EncryptionKeys: keys})
	require.NoError(t, krw.Write(cert, key, nil))
	dekManager, err := NewRaftDEKManager(krw)
	require.NoError(t, err)

	keyBytes, err := ioutil.ReadFile(paths.Node.Key)
	require.NoError(t, err)

	_, _, err = dekManager.MaybeUpdateKEK(ca.KEKData{KEK: []byte("locked now")})
	require.Error(t, err)
	require.False(t, dekManager.NeedsRotation())

	keyBytes2, err := ioutil.ReadFile(paths.Node.Key)
	require.NoError(t, err)
	require.Equal(t, keyBytes, keyBytes2)

	// trying to update a KEK from unlocked to lock will set NeedsRotation to true, as well as encrypt the TLS key
	updated, unlockedToLocked, err := dekManager.MaybeUpdateKEK(ca.KEKData{KEK: []byte("locked now"), Version: 1})
	require.NoError(t, err)
	require.True(t, updated)
	require.True(t, unlockedToLocked)
	// don't run GetKeys, because NeedsRotation is true and it'd just generate a new one
	h, _ := krw.GetCurrentState()
	dekData, ok := h.(RaftDEKData)
	require.True(t, ok)
	require.Equal(t, keys, dekData.EncryptionKeys)
	require.True(t, dekData.NeedsRotation)
	require.NotNil(t, <-dekManager.RotationNotify()) // we are notified of a new pending key

	keyBytes2, err = ioutil.ReadFile(paths.Node.Key)
	require.NoError(t, err)
	require.NotEqual(t, keyBytes, keyBytes2)
	keyBytes = keyBytes2

	readKRW := ca.NewKeyReadWriter(paths.Node, []byte("locked now"), RaftDEKData{})
	_, _, err = readKRW.Read()
	require.NoError(t, err)

	// trying to update a KEK of a lower version will not update anything, but will not error
	updated, unlockedToLocked, err = dekManager.MaybeUpdateKEK(ca.KEKData{})
	require.NoError(t, err)
	require.False(t, unlockedToLocked)
	require.False(t, updated)
	// don't run GetKeys, because NeedsRotation is true and it'd just generate a new one
	h, _ = krw.GetCurrentState()
	dekData, ok = h.(RaftDEKData)
	require.True(t, ok)
	require.Equal(t, keys, dekData.EncryptionKeys)
	require.True(t, dekData.NeedsRotation)

	keyBytes2, err = ioutil.ReadFile(paths.Node.Key)
	require.NoError(t, err)
	require.Equal(t, keyBytes, keyBytes2, string(keyBytes), string(keyBytes2))

	// updating a kek to a higher version, but with the same kek, will also neither update anything nor error
	updated, unlockedToLocked, err = dekManager.MaybeUpdateKEK(ca.KEKData{KEK: []byte("locked now"), Version: 100})
	require.NoError(t, err)
	require.False(t, unlockedToLocked)
	require.False(t, updated)
	// don't run GetKeys, because NeedsRotation is true and it'd just generate a new one
	h, _ = krw.GetCurrentState()
	dekData, ok = h.(RaftDEKData)
	require.True(t, ok)
	require.Equal(t, keys, dekData.EncryptionKeys)
	require.True(t, dekData.NeedsRotation)

	keyBytes2, err = ioutil.ReadFile(paths.Node.Key)
	require.NoError(t, err)
	require.Equal(t, keyBytes, keyBytes2)

	// going from locked to unlock does not result in the NeedsRotation flag, but does result in
	// the key being decrypted
	krw = ca.NewKeyReadWriter(paths.Node, []byte("kek"), RaftDEKData{EncryptionKeys: keys})
	require.NoError(t, krw.Write(cert, key, nil))
	dekManager, err = NewRaftDEKManager(krw)
	require.NoError(t, err)

	keyBytes, err = ioutil.ReadFile(paths.Node.Key)
	require.NoError(t, err)

	updated, unlockedToLocked, err = dekManager.MaybeUpdateKEK(ca.KEKData{Version: 2})
	require.NoError(t, err)
	require.False(t, unlockedToLocked)
	require.True(t, updated)
	require.Equal(t, keys, dekManager.GetKeys())
	require.False(t, dekManager.NeedsRotation())

	keyBytes2, err = ioutil.ReadFile(paths.Node.Key)
	require.NoError(t, err)
	require.NotEqual(t, keyBytes, keyBytes2)

	readKRW = ca.NewKeyReadWriter(paths.Node, nil, RaftDEKData{})
	_, _, err = readKRW.Read()
	require.NoError(t, err)
}

// The TLS KEK and the KEK for the headers should be in sync, and so failing
// to decrypt the TLS key should be mean we won't be able to decrypt the headers.
// However, the TLS Key encryption uses AES-256-CBC (golang as of 1.7.x does not seem
// to support GCM, so no cipher modes with digests) so sometimes decrypting with
// the wrong passphrase will not result in an error.  This means we will ultimately
// have to rely on the header encryption mechanism, which does include a digest, to
// determine if the KEK is valid.
func TestDecryptTLSKeyFalsePositive(t *testing.T) {
	badKey := []byte(`
-----BEGIN EC PRIVATE KEY-----
Proc-Type: 4,ENCRYPTED
DEK-Info: AES-256-CBC,e7927e79e748233776c03c2eb7275f09
kek-version: 392
raft-dek: CAESMBrzZ0gNVPe3FRs42743q8RtkUBrK1ICQpHWX2vdQ8iqSKt1WoKdFDFD2r28LYAVLxoYQguwHbijMx9k+BALUNBAI3s199S5tvnr

JfGenNvzm++AvsOh+UmcBY+JgI6lnfzaCB68agmlmEZYLYi5tqtAU7gif6VIJpCW
+Pj23Fzkw8sKKOOBeapSC5lp+Cjx9OsCci/R9xrdx+uxnnzKJNxOB/qzqcQfZDMh
id2LxdliFcPEk/Yj5gNGpT0UMFJ4G52enbOwOru46f0=
-----END EC PRIVATE KEY-----
`)

	// not actually a real swarm cert - generated a cert corresponding to the key that expires in 20 years
	matchingCert := []byte(`
-----BEGIN CERTIFICATE-----
MIIB9jCCAZygAwIBAgIRAIdzF3Z9VT2OXbRvEw5cR68wCgYIKoZIzj0EAwIwYDEi
MCAGA1UEChMZbWRwMXU5Z3FoOTV1NXN2MmNodDRrcDB1cTEWMBQGA1UECxMNc3dh
cm0tbWFuYWdlcjEiMCAGA1UEAxMZcXJzYmwza2FqOWhiZWprM2R5aWFlc3FiYTAg
GA8wMDAxMDEwMTAwMDAwMFoXDTM2MTEwODA2MjMwMlowYDEiMCAGA1UEChMZbWRw
MXU5Z3FoOTV1NXN2MmNodDRrcDB1cTEWMBQGA1UECxMNc3dhcm0tbWFuYWdlcjEi
MCAGA1UEAxMZcXJzYmwza2FqOWhiZWprM2R5aWFlc3FiYTBZMBMGByqGSM49AgEG
CCqGSM49AwEHA0IABGOivD25E/zcupRFQdKOKbPHS9Mx7JlUhlWnl0iR0K5VhVIU
XjUHt98GuX6gDjs4yUzEKSGxYPsSYlnG9zQqbQSjNTAzMA4GA1UdDwEB/wQEAwIF
oDATBgNVHSUEDDAKBggrBgEFBQcDATAMBgNVHRMBAf8EAjAAMAoGCCqGSM49BAMC
A0gAMEUCIQDWtjg1ITGznQILipaEe70G/NgZAOtFfuPXTVkUl3el+wIgSVOVKB/Q
O0T3aXuZGYNyh//KqAoA3erCmh6HauMz84Y=
-----END CERTIFICATE-----
	`)

	var wrongKEK []byte // empty passphrase doesn't decrypt without errors
	falsePositiveKEK, err := base64.RawStdEncoding.DecodeString("bIQgLAAMoGCrHdjMLVhEVqnYTAM7ZNF2xWMiwtw7AiQ")
	require.NoError(t, err)
	realKEK, err := base64.RawStdEncoding.DecodeString("fDg9YejLnMjU+FpulWR62oJLzVpkD2j7VQuP5xiK9QA")
	require.NoError(t, err)

	tempdir, err := ioutil.TempDir("", "KeyReadWriter-false-positive-decryption")
	require.NoError(t, err)
	defer os.RemoveAll(tempdir)

	path := ca.NewConfigPaths(tempdir)
	require.NoError(t, ioutil.WriteFile(path.Node.Key, badKey, 0600))
	require.NoError(t, ioutil.WriteFile(path.Node.Cert, matchingCert, 0644))

	krw := ca.NewKeyReadWriter(path.Node, wrongKEK, RaftDEKData{})
	_, _, err = krw.Read()
	require.IsType(t, ca.ErrInvalidKEK{}, errors.Cause(err))

	krw = ca.NewKeyReadWriter(path.Node, falsePositiveKEK, RaftDEKData{})
	_, _, err = krw.Read()
	require.Error(t, err)
	require.IsType(t, ca.ErrInvalidKEK{}, errors.Cause(err))

	krw = ca.NewKeyReadWriter(path.Node, realKEK, RaftDEKData{})
	_, _, err = krw.Read()
	require.NoError(t, err)
}
