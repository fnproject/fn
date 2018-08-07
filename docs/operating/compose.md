# Development environment deployment with Docker compose

### Deployment details

With [Docker Compose file](../../docker-compose.yml) it's easy to bootstrap the full Fn development stack that includes the following components:

 - MySQL server for database
 - Redis server for message queue
 - Fn server with scale group (supported only for Docker Swarm)
 - Fn UI
 - Grafana
 - Prometheus

Given compose file was developed specifically for Fn platform development/testing purpose, it's not a production-ready script. In order to get production deployment please review [Kubernetes Helm Chart for Fn](https://github.com/fnproject/fn-helm/).
