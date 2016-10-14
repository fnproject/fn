# OpenStack Ceilometer Example

This is an example how to call IronFunctions from OpenStack Ceilometer.

For simplicity, we will use vagrant & devstack.

It's assumed that execution file is located in this directory (`functions`).

## Install OpenStack

```bash
$ vagrant up
...
Horizon is now available at http://192.168.1.11/
Keystone is serving at http://192.168.1.11:5000/v2.0/
Examples on using novaclient command line is in exercise.sh
The default users are: admin and demo
The password: admin
This is your host ip: 192.168.1.11
```

## Run IronFunctions inside the 

Login into vagrant's box and run IronFunctions binary:

```bash
$ vagrant ssh
$ sudo /vagrant/functions
INFO[0000] creating new datastore                        db=bolt
INFO[0000] Creating bolt db at  /home/vagrant/devstack/data/bolt.db  db=bolt dir=/home/vagrant/devstack/data
INFO[0000] BoltDB initialized                            db=bolt dir=/home/vagrant/devstack/data file=/home/vagrant/devstack/data/bolt.db prefix=funcs
INFO[0000] selecting MQ                                  mq=bolt
INFO[0000] BoltDb initialized                            dir=/home/vagrant/devstack/data file=/home/vagrant/devstack/data/worker_mq.db mq=bolt
INFO[0000] async workers:1                              
[GIN-debug] [WARNING] Running in "debug" mode. Switch to "release" mode in production.
 - using env:   export GIN_MODE=release
 - using code:  gin.SetMode(gin.ReleaseMode)
...
```

Dont's exit this session. We will need this log later.


## Configure IronFunction

Login again and add some configuration for calling IronFunctions:

```bash
$ vagrant ssh
$ curl -H "Content-Type: application/json" -X POST -d '{"app": { "name":"myapp" }}' http://localhost:8080/v1/apps
{"message":"App successfully created","app":{"name":"myapp","config":null}}
$ curl -H "Content-Type: application/json" -X POST -d '{"route": {"path":"/hello","image":"iron/hello"}}' http://localhost:8080/v1/apps/myapp/routes
{"message":"Route successfully created","route":{"appname":"myapp","path":"/hello","image":"iron/hello","memory":128,"type":"sync","config":null}}
```

## Start an instance inside OpenStack

```bash
$ . devstack/openrc admin admin
$ IMAGE_ID=$(glance image-list | awk '/cirros-0.3.4-x86_64-uec / {print $2}')
$ nova boot --image $IMAGE_ID --flavor 42 test_alarms
$ nova list --all-tenants 
+--------------------------------------+-------------+----------------------------------+--------+------------+-------------+------------------+
| ID                                   | Name        | Tenant ID                        | Status | Task State | Power State | Networks         |
+--------------------------------------+-------------+----------------------------------+--------+------------+-------------+------------------+
| 7795a60f-f8f8-465b-a01b-7c23bc74e8d6 | test_alarms | 97d1e12123ea4d01b141cb7777e1e527 | ACTIVE | -          | Running     | private=10.1.1.2 |
+--------------------------------------+-------------+----------------------------------+--------+------------+-------------+------------------+
```

## Create an Alarm in OpenStack Ceilometer

```bash
$ ceilometer alarm-threshold-create \
		--name cpu_high \
		--description 'instance running hot' \
		--meter-name cpu_util  \
		--threshold 20.0 \
		--comparison-operator gt  \
		--statistic max \
		--period 600 \
		--evaluation-periods 1 \
		--alarm-action 'http://localhost:8080/r/myapp/hello' \
		--query resource_id=7795a60f-f8f8-465b-a01b-7c23bc74e8d6
$ ceilometer alarm-list 
+--------------------------------------+----------+-------+----------+---------+------------+--------------------------------------+------------------+
| Alarm ID                             | Name     | State | Severity | Enabled | Continuous | Alarm condition                      | Time constraints |
+--------------------------------------+----------+-------+----------+---------+------------+--------------------------------------+------------------+
| 9aae9489-0c9e-42ad-8fa7-23a2c68a8660 | cpu_high | ok    | low      | True    | False      | max(cpu_util) > 20.0 during 1 x 600s | None             |
+--------------------------------------+----------+-------+----------+---------+------------+--------------------------------------+------------------+
```

## Add some load to the instance

Login into just started instance:

```bash
host $ vagrant ssh
vagrant $ ssh cirros@10.1.1.2
cirros@10.1.1.2's password: 
$ dd if=/dev/zero of=/dev/null
```

## Checking IronFunctions log

In 5-10 minutes we should we in the IronFunctions's log:

```
INFO[1633]                                               name=run.myapp.requests type=count value=1
INFO[1633]                                               name=run.myapp.waittime type=time value=0s
[GIN] 2016/10/14 - 12:43:38 | 202 |      47.232µs | 127.0.0.1 |   GET     /tasks
INFO[1633]                                               name=run.myapp.succeeded type=count value=1
INFO[1633]                                               name=run.myapp.time type=time value=240.384458ms
INFO[1633]                                               name=run.exec_time type=time value=240.384458ms
[GIN] 2016/10/14 - 12:43:39 | 200 |   985.96401ms | 127.0.0.1 |   POST    /r/myapp/hello
```

Hooray! `myapp/hello` was called by OpenStack Ceilometer!
