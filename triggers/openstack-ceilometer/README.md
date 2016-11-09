# OpenStack Ceilometer Example

This is an example of using OpenStack Ceilometer notifications as an event
source for IronFunctions.

For simplicity, we will use [vagrant](https://github.com/mitchellh/vagrant) & [devstack](https://github.com/openstack-dev/devstack).

It's assumed that IronFunctions is built and located in this directory (`functions`).

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

## Run IronFunctions inside the VM

Login to Vagrant instance and start IronFunctions:

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

Don't exit this session. We will need this log later.


## Configure IronFunctions

Login again and add some configuration for calling IronFunctions:

```bash
$ vagrant ssh
$ curl -H "Content-Type: application/json" -X POST -d '{"app": { "name":"myapp" }}' http://localhost:8080/v1/apps
{"message":"App successfully created","app":{"name":"myapp","config":null}}
$ curl -H "Content-Type: application/json" -X POST -d '{"route": {"path":"/hello","image":"iron/hello"}}' http://localhost:8080/v1/apps/myapp/routes
{"message":"Route successfully created","route":{"appname":"myapp","path":"/hello","image":"iron/hello","memory":128,"type":"sync","config":null}}
```

## Start a Nova compute instance inside OpenStack

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

## Use the OpenStack Ceilometer CLI to create an alarm threshold

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

## (Optional): Change the Ceilometer polling interval

The default value for the Ceilometer polling interval is 600 seconds. For the purpose of this example let's change it to 60 seconds.

```bash
$ sed -i -- 's/interval: 600/interval: 60/g' /etc/ceilometer/pipeline.yaml
```

Next we need to restart some Ceilometer services:

  * ceilometer-acentral
  * ceilometer-collector
  * ceilometer-acompute

In Devstack, all OpenStack services are running under
[screen](https://en.wikipedia.org/wiki/Screen). Thus, to restart processes
listed above we need to do:

  * `screen -r` to detach running screen session

  * press `Ctrl+A` and then `Shift+'`, you will see a list of windows
  * select window #17 `ceilometer-acentral`
  * press `Ctrl+c` to stop current proccess
  * press `up-arrow` to select previous command
  * press `Enter` to start it again with a new config

  * press `Ctrl+A` and then `Shift+'`, you will see a list of windows
  * select window #22 `ceilometer-collector`
  * press `Ctrl+c` to stop current proccess
  * press `up-arrow` to select previous command
  * press `Enter` to start it again with a new config

  * press `Ctrl+A` and then `Shift+'`, you will see a list of windows
  * select window #23 `ceilometer-acompute`
  * press `Ctrl+c` to stop current proccess
  * press `up-arrow` to select previous command
  * press `Enter` to start it again with a new config

Now we should see more frequent samples, for example:

```bash
$ ceilometer sample-list --meter cpu
+--------------------------------------+------+------------+---------------+------+----------------------------+
| Resource ID                          | Name | Type       | Volume        | Unit | Timestamp                  |
+--------------------------------------+------+------------+---------------+------+----------------------------+
| 35391242-4b40-43cb-a18d-0b0438282c0c | cpu  | cumulative | 13770000000.0 | ns   | 2016-10-14T12:37:49.768882 |
| 35391242-4b40-43cb-a18d-0b0438282c0c | cpu  | cumulative | 13680000000.0 | ns   | 2016-10-14T12:36:49.868174 |
| 35391242-4b40-43cb-a18d-0b0438282c0c | cpu  | cumulative | 13570000000.0 | ns   | 2016-10-14T12:35:50.206048 |
| 35391242-4b40-43cb-a18d-0b0438282c0c | cpu  | cumulative | 13150000000.0 | ns   | 2016-10-14T12:27:12.947970 |
| 35391242-4b40-43cb-a18d-0b0438282c0c | cpu  | cumulative | 12260000000.0 | ns   | 2016-10-14T12:17:13.246754 |
+--------------------------------------+------+------------+---------------+------+----------------------------+
```


## Trigger the alarm we created in the previous step by adding load to the instance

Login to Nova compute instance we created in previous step:

```bash
host $ vagrant ssh
vagrant $ ssh cirros@10.1.1.2
cirros@10.1.1.2's password: 
$ dd if=/dev/zero of=/dev/null
```

## Checking IronFunctions log

In 1-2 minutes the Ceilometer alarm will trigger an HTTP callback to the
IronFunctions route we created earlier, and this can be seen from the
IronFunctions API server log:

```
INFO[1633]                                               name=run.myapp.requests type=count value=1
INFO[1633]                                               name=run.myapp.waittime type=time value=0s
[GIN] 2016/10/14 - 12:43:38 | 202 |      47.232µs | 127.0.0.1 |   GET     /tasks
INFO[1633]                                               name=run.myapp.succeeded type=count value=1
INFO[1633]                                               name=run.myapp.time type=time value=240.384458ms
INFO[1633]                                               name=run.exec_time type=time value=240.384458ms
[GIN] 2016/10/14 - 12:43:39 | 200 |   985.96401ms | 127.0.0.1 |   POST    /r/myapp/hello
```

Hooray! `myapp/hello` was triggered by OpenStack Ceilometer!
