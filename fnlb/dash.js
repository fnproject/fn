
var example = 'dynamic-update', 
  theme = 'default';

var chart; // global
var seriesMapper = {}; // map by node+func name to chart[i] index

Highcharts.setOptions({
  global: {
    useUTC: false
  }
});

function requestData() {
  $.ajax({
    url: '/1/lb/stats',
    success: function(point) {
      var jason = JSON.parse(point);

      if (!jason["stats"] || jason["stats"].length == 0) {
        // XXX (reed): using server timestamps for real data this can drift easily
        // XXX (reed): uh how to insert empty data point w/o node data? enum all series names?
        //series.addPoint([(new Date()).getTime(), 0], true, shift);
        //series: [{
        //name: 'Random data',
        //data: []
        //}]
        setTimeout(requestData, 1000);
        return
      }

      for (var i = 0; i < jason["stats"].length; i++) {
        stat = jason["stats"][i];
        var node = stat["node"];
        var func = stat["func"];
        var key = node + func

        if (seriesMapper[key] == null) {
          chart.addSeries({name: key, data: []});
          waitChart.addSeries({name: key, data: []});
          seriesMapper[key] = chart.series.length - 1;
        }

        series = chart.series[seriesMapper[key]];
        waitSeries = waitChart.series[seriesMapper[key]];

        // XXX (reed): hack
        shift = series.data.length > 20 && i == jason["stats"].length + 1;

        timestamp = Date.parse(stat["timestamp"]);
        console.log(series.data.length, timestamp, stat["tp"], stat["wait"]);
        series.addPoint([timestamp, stat["tp"]], false, shift);
        waitSeries.addPoint([timestamp, stat["wait"]], false, shift);
      }
      if (jason["stats"].length > 0) {
        chart.redraw();
        waitChart.redraw();
      }

      // call it again after one second
      // XXX (reed): this won't work perfectly cuz if the endpoint fails then we won't ask for more datas
      setTimeout(requestData, 1000);
    },
    cache: false
  });
}

$(document).ready(function() {
  chart = new Highcharts.Chart({
    chart: {
      renderTo: 'throughput_chart',
      events: {
        load: requestData
      }
    },
    rangeSelector: {
      buttons: [{
        count: 1,
        type: 'minute',
        text: '1M'
      }, {
        count: 5,
        type: 'minute',
        text: '5M'
      }, {
        type: 'all',
        text: 'All'
      }],
      //inputEnabled: false,
      selected: 0
    },
    title: {
      text: 'lb data'
    },
    exporting: {
      enabled: false
    },
    xAxis: {
      type: 'datetime',
      tickPixelInterval: 150,
      maxZoom: 20 * 1000
    },
    yAxis: {
      minPadding: 0.2,
      maxPadding: 0.2,
      title: {
        text: 'throughput (/s)',
        margin: 80
      }
    },
    series: []
  });

  waitChart = new Highcharts.Chart({
    chart: {
      renderTo: 'wait_chart',
      events: {
        load: requestData
      }
    },
    rangeSelector: {
      buttons: [{
        count: 1,
        type: 'minute',
        text: '1M'
      }, {
        count: 5,
        type: 'minute',
        text: '5M'
      }, {
        type: 'all',
        text: 'All'
      }],
      //inputEnabled: false,
      selected: 0
    },
    title: {
      text: 'lb data'
    },
    exporting: {
      enabled: false
    },
    xAxis: {
      type: 'datetime',
      tickPixelInterval: 150,
      maxZoom: 20 * 1000
    },
    yAxis: {
      minPadding: 0.2,
      maxPadding: 0.2,
      title: {
        text: 'wait time (seconds)',
        margin: 80
      }
    },
    series: []
  });
});
