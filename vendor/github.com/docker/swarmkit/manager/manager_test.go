package manager

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/net/context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/ca"
	cautils "github.com/docker/swarmkit/ca/testutils"
	"github.com/docker/swarmkit/manager/dispatcher"
	"github.com/docker/swarmkit/manager/encryption"
	"github.com/docker/swarmkit/manager/state/raft/storage"
	"github.com/docker/swarmkit/manager/state/store"
	"github.com/docker/swarmkit/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager(t *testing.T) {
	temp, err := ioutil.TempFile("", "test-socket")
	assert.NoError(t, err)
	assert.NoError(t, temp.Close())
	assert.NoError(t, os.Remove(temp.Name()))

	defer os.RemoveAll(temp.Name())

	stateDir, err := ioutil.TempDir("", "test-raft")
	assert.NoError(t, err)
	defer os.RemoveAll(stateDir)

	tc := cautils.NewTestCA(t, func(p ca.CertPaths) *ca.KeyReadWriter {
		return ca.NewKeyReadWriter(p, []byte("kek"), nil)
	})
	defer tc.Stop()

	agentSecurityConfig, err := tc.NewNodeConfig(ca.WorkerRole)
	assert.NoError(t, err)
	agentDiffOrgSecurityConfig, err := tc.NewNodeConfigOrg(ca.WorkerRole, "another-org")
	assert.NoError(t, err)
	managerSecurityConfig, err := tc.NewNodeConfig(ca.ManagerRole)
	assert.NoError(t, err)

	m, err := New(&Config{
		RemoteAPI:        &RemoteAddrs{ListenAddr: "127.0.0.1:0"},
		ControlAPI:       temp.Name(),
		StateDir:         stateDir,
		SecurityConfig:   managerSecurityConfig,
		AutoLockManagers: true,
		UnlockKey:        []byte("kek"),
		RootCAPaths:      tc.Paths.RootCA,
	})
	assert.NoError(t, err)
	assert.NotNil(t, m)

	tcpAddr := m.Addr()

	done := make(chan error)
	defer close(done)
	go func() {
		done <- m.Run(tc.Context)
	}()

	opts := []grpc.DialOption{
		grpc.WithTimeout(10 * time.Second),
		grpc.WithTransportCredentials(agentSecurityConfig.ClientTLSCreds),
	}

	conn, err := grpc.Dial(tcpAddr, opts...)
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, conn.Close())
	}()

	// We have to send a dummy request to verify if the connection is actually up.
	client := api.NewDispatcherClient(conn)
	_, err = client.Heartbeat(tc.Context, &api.HeartbeatRequest{})
	assert.Equal(t, dispatcher.ErrNodeNotRegistered.Error(), grpc.ErrorDesc(err))
	_, err = client.Session(tc.Context, &api.SessionRequest{})
	assert.NoError(t, err)

	// Try to have a client in a different org access this manager
	opts = []grpc.DialOption{
		grpc.WithTimeout(10 * time.Second),
		grpc.WithTransportCredentials(agentDiffOrgSecurityConfig.ClientTLSCreds),
	}

	conn2, err := grpc.Dial(tcpAddr, opts...)
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, conn2.Close())
	}()

	client = api.NewDispatcherClient(conn2)
	_, err = client.Heartbeat(context.Background(), &api.HeartbeatRequest{})
	assert.Contains(t, grpc.ErrorDesc(err), "Permission denied: unauthorized peer role: rpc error: code = PermissionDenied desc = Permission denied: remote certificate not part of organization")

	// Verify that requests to the various GRPC services running on TCP
	// are rejected if they don't have certs.
	opts = []grpc.DialOption{
		grpc.WithTimeout(10 * time.Second),
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})),
	}

	noCertConn, err := grpc.Dial(tcpAddr, opts...)
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, noCertConn.Close())
	}()

	client = api.NewDispatcherClient(noCertConn)
	_, err = client.Heartbeat(context.Background(), &api.HeartbeatRequest{})
	assert.EqualError(t, err, "rpc error: code = PermissionDenied desc = Permission denied: unauthorized peer role: rpc error: code = PermissionDenied desc = no client certificates in request")

	controlClient := api.NewControlClient(noCertConn)
	_, err = controlClient.ListNodes(context.Background(), &api.ListNodesRequest{})
	assert.EqualError(t, err, "rpc error: code = PermissionDenied desc = Permission denied: unauthorized peer role: rpc error: code = PermissionDenied desc = no client certificates in request")

	raftClient := api.NewRaftMembershipClient(noCertConn)
	_, err = raftClient.Join(context.Background(), &api.JoinRequest{})
	assert.EqualError(t, err, "rpc error: code = PermissionDenied desc = Permission denied: unauthorized peer role: rpc error: code = PermissionDenied desc = no client certificates in request")

	opts = []grpc.DialOption{
		grpc.WithTimeout(10 * time.Second),
		grpc.WithTransportCredentials(managerSecurityConfig.ClientTLSCreds),
	}

	controlConn, err := grpc.Dial(tcpAddr, opts...)
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, controlConn.Close())
	}()

	// check that the kek is added to the config
	var cluster api.Cluster
	require.NoError(t, testutils.PollFunc(nil, func() error {
		var (
			err      error
			clusters []*api.Cluster
		)
		m.raftNode.MemoryStore().View(func(tx store.ReadTx) {
			clusters, err = store.FindClusters(tx, store.All)
		})
		if err != nil {
			return err
		}
		if len(clusters) != 1 {
			return errors.New("wrong number of clusters")
		}
		cluster = *clusters[0]
		return nil

	}))
	require.NotNil(t, cluster)
	require.Len(t, cluster.UnlockKeys, 1)
	require.Equal(t, &api.EncryptionKey{
		Subsystem: ca.ManagerRole,
		Key:       []byte("kek"),
	}, cluster.UnlockKeys[0])

	// Test removal of the agent node
	agentID := agentSecurityConfig.ClientTLSCreds.NodeID()
	assert.NoError(t, m.raftNode.MemoryStore().Update(func(tx store.Tx) error {
		return store.CreateNode(tx,
			&api.Node{
				ID: agentID,
				Certificate: api.Certificate{
					Role: api.NodeRoleWorker,
					CN:   agentID,
				},
			},
		)
	}))
	controlClient = api.NewControlClient(controlConn)
	_, err = controlClient.CreateNetwork(context.Background(), &api.CreateNetworkRequest{
		Spec: &api.NetworkSpec{
			Annotations: api.Annotations{
				Name: "test-network-bad-driver",
			},
			DriverConfig: &api.Driver{
				Name: "invalid-must-never-exist",
			},
		},
	})
	assert.Error(t, err)

	_, err = controlClient.RemoveNode(context.Background(),
		&api.RemoveNodeRequest{
			NodeID: agentID,
			Force:  true,
		},
	)
	assert.NoError(t, err)

	client = api.NewDispatcherClient(conn)
	_, err = client.Heartbeat(context.Background(), &api.HeartbeatRequest{})
	assert.Contains(t, grpc.ErrorDesc(err), "removed from swarm")

	m.Stop(tc.Context, false)

	// After stopping we should MAY receive an error from ListenAndServe if
	// all this happened before WaitForLeader completed, so don't check the
	// error.
	<-done
}

// Tests locking and unlocking the manager and key rotations
func TestManagerLockUnlock(t *testing.T) {
	temp, err := ioutil.TempFile("", "test-manager-lock")
	require.NoError(t, err)
	require.NoError(t, temp.Close())
	require.NoError(t, os.Remove(temp.Name()))

	defer os.RemoveAll(temp.Name())

	stateDir, err := ioutil.TempDir("", "test-raft")
	require.NoError(t, err)
	defer os.RemoveAll(stateDir)

	tc := cautils.NewTestCA(t)
	defer tc.Stop()

	managerSecurityConfig, err := tc.NewNodeConfig(ca.ManagerRole)
	require.NoError(t, err)

	_, _, err = managerSecurityConfig.KeyReader().Read()
	require.NoError(t, err)

	m, err := New(&Config{
		RemoteAPI:      &RemoteAddrs{ListenAddr: "127.0.0.1:0"},
		ControlAPI:     temp.Name(),
		StateDir:       stateDir,
		SecurityConfig: managerSecurityConfig,
		RootCAPaths:    tc.Paths.RootCA,
		// start off without any encryption
	})
	require.NoError(t, err)
	require.NotNil(t, m)

	done := make(chan error)
	defer close(done)
	go func() {
		done <- m.Run(tc.Context)
	}()

	opts := []grpc.DialOption{
		grpc.WithTimeout(10 * time.Second),
		grpc.WithTransportCredentials(managerSecurityConfig.ClientTLSCreds),
	}

	conn, err := grpc.Dial(m.Addr(), opts...)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, conn.Close())
	}()

	// check that there is no kek currently - we are using the API because this
	// lets us wait until the manager is up and listening, as well
	var cluster *api.Cluster
	client := api.NewControlClient(conn)

	require.NoError(t, testutils.PollFuncWithTimeout(nil, func() error {
		resp, err := client.ListClusters(tc.Context, &api.ListClustersRequest{})
		if err != nil {
			return err
		}
		if len(resp.Clusters) == 0 {
			return fmt.Errorf("no clusters yet")
		}
		cluster = resp.Clusters[0]
		return nil
	}, 1*time.Second))

	require.Nil(t, cluster.UnlockKeys)

	// tls key is unencrypted, but there is a DEK
	key, err := ioutil.ReadFile(tc.Paths.Node.Key)
	require.NoError(t, err)
	keyBlock, _ := pem.Decode(key)
	require.NotNil(t, keyBlock)
	require.False(t, x509.IsEncryptedPEMBlock(keyBlock))
	require.Len(t, keyBlock.Headers, 2)
	currentDEK, err := decodePEMHeaderValue(keyBlock.Headers[pemHeaderRaftDEK], nil)
	require.NoError(t, err)
	require.NotEmpty(t, currentDEK)

	// update the lock key - this may fail due to update out of sequence errors, so try again
	for {
		getResp, err := client.GetCluster(tc.Context, &api.GetClusterRequest{ClusterID: cluster.ID})
		require.NoError(t, err)
		cluster = getResp.Cluster

		spec := cluster.Spec.Copy()
		spec.EncryptionConfig.AutoLockManagers = true
		updateResp, err := client.UpdateCluster(tc.Context, &api.UpdateClusterRequest{
			ClusterID:      cluster.ID,
			ClusterVersion: &cluster.Meta.Version,
			Spec:           spec,
		})
		if grpc.ErrorDesc(err) == "update out of sequence" {
			continue
		}
		// if there is any other type of error, this should fail
		if err == nil {
			cluster = updateResp.Cluster
		}
		break
	}
	require.NoError(t, err)

	caConn := api.NewCAClient(conn)
	unlockKeyResp, err := caConn.GetUnlockKey(tc.Context, &api.GetUnlockKeyRequest{})
	require.NoError(t, err)

	// this should update the TLS key, rotate the DEK, and finish snapshotting
	var updatedKey []byte
	require.NoError(t, testutils.PollFuncWithTimeout(nil, func() error {
		updatedKey, err = ioutil.ReadFile(tc.Paths.Node.Key)
		require.NoError(t, err) // this should never error due to atomic writes

		if bytes.Equal(key, updatedKey) {
			return fmt.Errorf("TLS key should have been re-encrypted at least")
		}

		keyBlock, _ = pem.Decode(updatedKey)
		require.NotNil(t, keyBlock) // this should never error due to atomic writes

		if !x509.IsEncryptedPEMBlock(keyBlock) {
			return fmt.Errorf("Key not encrypted")
		}

		// we don't check that the TLS key has been rotated, because that may take
		// a little bit, and is best effort only

		currentDEKString, ok := keyBlock.Headers[pemHeaderRaftDEK]
		require.True(t, ok) // there should never NOT be a current header
		nowCurrentDEK, err := decodePEMHeaderValue(currentDEKString, unlockKeyResp.UnlockKey)
		require.NoError(t, err) // it should always be encrypted
		if bytes.Equal(currentDEK, nowCurrentDEK) {
			return fmt.Errorf("snapshot has not been finished yet")
		}

		currentDEK = nowCurrentDEK
		return nil
	}, 1*time.Second))

	_, ok := keyBlock.Headers[pemHeaderRaftPendingDEK]
	require.False(t, ok) // once the snapshot is do

	_, ok = keyBlock.Headers[pemHeaderRaftDEKNeedsRotation]
	require.False(t, ok)

	// verify that the snapshot is readable with the new DEK
	encrypter, decrypter := encryption.Defaults(currentDEK)
	// we can't use the raftLogger, because the WALs are still locked while the raft node is up.  And once we remove
	// the manager, they'll be deleted.
	snapshot, err := storage.NewSnapFactory(encrypter, decrypter).New(filepath.Join(stateDir, "raft", "snap-v3-encrypted")).Load()
	require.NoError(t, err)
	require.NotNil(t, snapshot)

	// update the lock key to nil
	for i := 0; i < 3; i++ {
		getResp, err := client.GetCluster(tc.Context, &api.GetClusterRequest{ClusterID: cluster.ID})
		require.NoError(t, err)
		cluster = getResp.Cluster

		spec := cluster.Spec.Copy()
		spec.EncryptionConfig.AutoLockManagers = false
		_, err = client.UpdateCluster(tc.Context, &api.UpdateClusterRequest{
			ClusterID:      cluster.ID,
			ClusterVersion: &cluster.Meta.Version,
			Spec:           spec,
		})
		if grpc.ErrorDesc(err) == "update out of sequence" {
			continue
		}
		require.NoError(t, err)
	}

	// this should update the TLS key
	var unlockedKey []byte
	require.NoError(t, testutils.PollFuncWithTimeout(nil, func() error {
		unlockedKey, err = ioutil.ReadFile(tc.Paths.Node.Key)
		if err != nil {
			return err
		}

		if bytes.Equal(unlockedKey, updatedKey) {
			return fmt.Errorf("TLS key should have been rotated")
		}

		return nil
	}, 1*time.Second))

	// the new key should not be encrypted, and the DEK should also be unencrypted
	// but not rotated
	keyBlock, _ = pem.Decode(unlockedKey)
	require.NotNil(t, keyBlock)
	require.False(t, x509.IsEncryptedPEMBlock(keyBlock))

	unencryptedDEK, err := decodePEMHeaderValue(keyBlock.Headers[pemHeaderRaftDEK], nil)
	require.NoError(t, err)
	require.NotNil(t, unencryptedDEK)
	require.Equal(t, currentDEK, unencryptedDEK)

	m.Stop(tc.Context, false)

	// After stopping we should MAY receive an error from ListenAndServe if
	// all this happened before WaitForLeader completed, so don't check the
	// error.
	<-done
}

// Tests manager rotates encryption of root key data in the raft store
func TestManagerEncryptsDecryptsRootKeyMaterial(t *testing.T) {
	tc := cautils.NewTestCA(t)
	defer tc.Stop()

	temp, err := ioutil.TempFile("", "test-socket")
	require.NoError(t, err)
	require.NoError(t, temp.Close())
	require.NoError(t, os.Remove(temp.Name()))

	defer os.RemoveAll(temp.Name())

	stateDir, err := ioutil.TempDir("", "test-raft")
	require.NoError(t, err)
	defer os.RemoveAll(stateDir)

	managerSecurityConfig, err := tc.NewNodeConfig(ca.ManagerRole)
	require.NoError(t, err)

	_, _, err = managerSecurityConfig.KeyReader().Read()
	require.NoError(t, err)

	config := Config{
		RemoteAPI:      &RemoteAddrs{ListenAddr: "127.0.0.1:0"},
		ControlAPI:     temp.Name(),
		StateDir:       stateDir,
		SecurityConfig: managerSecurityConfig,
		RootCAPaths:    tc.Paths.RootCA,
	}
	done := make(chan error)
	defer close(done)

	var m *Manager
	startManager := func() {
		m, err = New(&config)
		require.NoError(t, err)
		require.NotNil(t, m)

		go func() {
			done <- m.Run(tc.Context)
		}()
	}

	startManager()

	var clusterID string
	// wait for cluster data to be there
	err = testutils.PollFunc(nil, func() error {
		// using store.Update just because it returns an error, as opposed to store.View
		return m.raftNode.MemoryStore().Update(func(tx store.Tx) error {
			clusters, err := store.FindClusters(tx, store.All)
			if err != nil {
				return err
			}
			if len(clusters) != 1 {
				return fmt.Errorf("expected 1 cluster, got %d", len(clusters))
			}
			clusterID = clusters[0].ID
			return nil
		})
	})

	os.Setenv(ca.PassphraseENVVar, "kek")
	defer os.Unsetenv(ca.PassphraseENVVar)

	// restart
	m.Stop(tc.Context, false)
	<-done
	startManager()

	// wait for the key to be encrypted in the raft store
	err = testutils.PollFunc(nil, func() error {
		return m.raftNode.MemoryStore().Update(func(tx store.Tx) error {
			cluster := store.GetCluster(tx, clusterID)
			if cluster == nil {
				return fmt.Errorf("cluster gone")
			}
			keyBlock, _ := pem.Decode(cluster.RootCA.CAKey)
			if keyBlock == nil {
				return fmt.Errorf("could not pem decode root key")
			}
			if !x509.IsEncryptedPEMBlock(keyBlock) {
				return fmt.Errorf("root key material not encrypted yet")
			}
			_, err = x509.DecryptPEMBlock(keyBlock, []byte("kek"))
			return err
		})
	})
	require.NoError(t, err)

	os.Unsetenv(ca.PassphraseENVVar)
	os.Setenv(ca.PassphraseENVVarPrev, "kek")
	defer os.Unsetenv(ca.PassphraseENVVarPrev)

	// restart
	m.Stop(tc.Context, false)
	<-done
	startManager()

	// wait for the key to be decrypted in the raft store
	pollDecrypted := func() error {
		return testutils.PollFunc(nil, func() error {
			// wait until we are leader first, because otherwise the raft node could still be catching
			// up on all the logs on disk and hence not have processed the "encrypt CA key" log yet
			if !m.raftNode.IsLeader() {
				return fmt.Errorf("node is not leader yet")
			}
			return m.raftNode.MemoryStore().Update(func(tx store.Tx) error {
				cluster := store.GetCluster(tx, clusterID)
				if cluster == nil {
					return fmt.Errorf("cluster gone")
				}
				keyBlock, _ := pem.Decode(cluster.RootCA.CAKey)
				if keyBlock == nil {
					return fmt.Errorf("could not pem decode root key")
				}
				if x509.IsEncryptedPEMBlock(keyBlock) {
					return fmt.Errorf("root key material not decrypted yet")
				}
				return nil
			})
		})
	}
	require.NoError(t, pollDecrypted())

	// update the key to that can be "decrypted" with both "" and "kek" as the password.  This
	// doesn't actually match the root CA certificate, and hence the security config can't be
	// updated, but we're just checking that the CA key is decrypted.
	require.NoError(t, m.raftNode.MemoryStore().Update(func(tx store.Tx) error {
		cluster := store.GetCluster(tx, clusterID)
		if cluster == nil {
			return fmt.Errorf("cluster gone")
		}
		cluster.RootCA.CAKey = []byte(`
-----BEGIN EC PRIVATE KEY-----
Proc-Type: 4,ENCRYPTED
DEK-Info: AES-256-CBC,fcc97c79c251d2fedeab96a19f3b826e

8IHMsMKfCMWXDpBNLp7tyuwUQ1FmisiPyDZg9UvoX4RvIDUxj7sIiw4lsP+EgnKG
09oKeXHSYRpawB58dvLqxPtjnrEj1jLqoMydTrhRDJ+zBMxPxpTJh/BASADhMOmf
G80TfNRRr/qdB9hLwfyOyk2tBipkAgs6cl+CZAaqx3k=
-----END EC PRIVATE KEY-----
`)
		return store.UpdateCluster(tx, cluster)
	}))

	// restart
	m.Stop(tc.Context, false)
	<-done
	startManager()
	require.NoError(t, pollDecrypted())

	m.Stop(tc.Context, false)
	<-done
}
