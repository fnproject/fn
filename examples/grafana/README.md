# Display runtime metrics using Prometheus and Grafana

The Fn server exports metrics using [Prometheus](https://prometheus.io/). This allows [Grafana](https://grafana.com/) to be used to display these metrics graphically. 

<img src="../../docs/assets/GrafanaDashboard.png" width="800">

## Start an Fn server and deploy some functions

This example requires an Fn server to be running and that you have deployed one or more functions. 
See the [front page](/README.md) or any of the other examples for instructions. 

The steps below assume that the Fn server is running at `localhost:8080`.

## Examine the endpoint used to export metrics to Prometheus

The Fn server exports metrics to Prometheus using the API endpoint `/metrics`. 

Try pointing your browser at [http://localhost:8080/metrics](http://localhost:8080/metrics).
This will display the metrics in prometheus format.

## Start Prometheus

Open a terminal window and navigate to the directory containing this example.

Examine the provised Prometheus configuration file:

```
cat prometheus.yml
```

This gives

``` yml
global:
  scrape_interval:     15s # By default, scrape targets every 15 seconds.

  # Attach these labels to any time series or alerts when communicating with
  # external systems (federation, remote storage, Alertmanager).
  external_labels:
    monitor: 'fn-monitor'

# A scrape configuration containing exactly one endpoint to scrape:
# Here it's the Fn server
scrape_configs:
  # The job name is added as a label `job=<job_name>` to any timeseries scraped from this config.
  - job_name: 'functions'

    # Override the global default and scrape targets from this job every 5 seconds.
    scrape_interval: 5s

    static_configs:
      # Specify all the Fn servers from which metrics will be scraped
      - targets: ['localhost:8080'] # Uses /metrics by default
```
Note the last line. This specifies the host and port of the Fn server from which metrics will be obtained. 
If you are running a cluster of Fn servers then you can specify them all here.

Now start Prometheus, specifying this config file:
```
docker run --name=prometheus -d -p 9090:9090 \
  --mount type=bind,source=`pwd`/prometheus.yml,target=/etc/prometheus/prometheus.yml \
  --add-host="localhost:`route | grep default | awk '{print $2}'`" prom/prometheus
```
Note: The parameter `` --add-host="localhost:`route | grep default | awk '{print $2}'`" `` means that Prometheus can use localhost to refer to the host. (The expression `` `route | grep default | awk '{print $2}'` ``  returns the IP of the host).

Open a browser on Prometheus's graph tool at [http://localhost:9090/graph](http://localhost:9090/graph). If you wish you can use this to view metrics and display metrics from the Fn server: see the [Prometheus](https://prometheus.io/) documentation for instructions. Alternatively continue with the next step to view a ready-made set of graphs in Grafana.

## Start Grafana and load the example dashboard

[Grafana](https://grafana.com/) provides powerful and flexible facilities to create graphs of any metric available to Prometheus. This example provides a ready-made dashboard that displays the numbers of functions that are queued, running, completed and failed. 

Open a terminal window and navigate to the directory containing this example.

Start Grafana on port 3000:
```
docker run --name=grafana -d -p 3000:3000 \
  --add-host="localhost:`route | grep default | awk '{print $2}'`" grafana/grafana
```

Open a browser on Grafana at [http://localhost:3000](http://localhost:3000).

Login using the default user `admin` and default password `admin`.

Create a datasource to obtain metrics from Promethesus:
* Click on **Add data source**. In the form that opens:
* Set **Name** to `PromDS` (or whatever name you choose)
* Set **Type** to `Prometheus`
* Set **URL** to `http://localhost:9090` 
* Set **Access** to `proxy`
* Click **Add** and then **Save and test**

Import the example dashboard that displays metrics from the Fn server:
* Click on the main menu at the top left and choose **Dashboards** and then **Home**
* Click on **Home** at the top and then **Import dashboard**
* In the dialog that opens, click **Upload .json file** and specify `fn_grafana_dashboard.json` in this example's directory.
* Specify the Prometheus data source that you just created
* Click **Import**

You should then see the dashboard shown above. Now execute some functions and see the graphs update.

