package envconfig

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMakeAllPossibleKeys(t *testing.T) {
	fieldName := "CassandraSslCert"
	keys := makeAllPossibleKeys(&context{
		name: fieldName,
	})

	require.Equal(t, 4, len(keys))
	require.Equal(t, "CASSANDRASSLCERT", keys[0])
	require.Equal(t, "CASSANDRA_SSL_CERT", keys[1])
	require.Equal(t, "cassandra_ssl_cert", keys[2])
	require.Equal(t, "cassandrasslcert", keys[3])

	fieldName = "CassandraSSLCert"
	keys = makeAllPossibleKeys(&context{
		name: fieldName,
	})

	require.Equal(t, 4, len(keys))
	require.Equal(t, "CASSANDRASSLCERT", keys[0])
	require.Equal(t, "CASSANDRA_SSL_CERT", keys[1])
	require.Equal(t, "cassandra_ssl_cert", keys[2])
	require.Equal(t, "cassandrasslcert", keys[3])

	fieldName = "Cassandra.SslCert"
	keys = makeAllPossibleKeys(&context{
		name: fieldName,
	})

	require.Equal(t, 4, len(keys))
	require.Equal(t, "CASSANDRA_SSLCERT", keys[0])
	require.Equal(t, "CASSANDRA_SSL_CERT", keys[1])
	require.Equal(t, "cassandra_ssl_cert", keys[2])
	require.Equal(t, "cassandra_sslcert", keys[3])

	fieldName = "Cassandra.SSLCert"
	keys = makeAllPossibleKeys(&context{
		name: fieldName,
	})

	require.Equal(t, 4, len(keys))
	require.Equal(t, "CASSANDRA_SSLCERT", keys[0])
	require.Equal(t, "CASSANDRA_SSL_CERT", keys[1])
	require.Equal(t, "cassandra_ssl_cert", keys[2])
	require.Equal(t, "cassandra_sslcert", keys[3])

	fieldName = "Name"
	keys = makeAllPossibleKeys(&context{
		name: fieldName,
	})

	require.Equal(t, 2, len(keys))
	require.Equal(t, "NAME", keys[0])
	require.Equal(t, "name", keys[1])
}
