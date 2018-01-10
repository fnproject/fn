# Development environment deployment with Docker compose

### Deployment details

With [Docker Compose file](../../docker-compose.yml) it's easy to bootstrap the full Fn development stack that includes the following components:

 - MySQL InnoDB cluster initiator
 - MySQL InnoDB cluster member
 - MySQL InnoDB cluster member
 - MySQL InnoDB cluster router
 - Redis server for message queue
 - Fn server 1
 - Fn server 2
 - Fn server 3
 - Fn loadbalancer
 - Fn UI
 - Grafana
 - Prometheus

Given compose file was developed specifically for Fn platform development/testing purpose, it's not a production-ready script. In order to get production deployment please review [Kubernetes deployment instruction](./kubernetes/README.md).
