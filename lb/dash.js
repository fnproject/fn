
var example = 'dynamic-update', 
  theme = 'default';

var chart; // global
var seriesMapper = {};

Highcharts.setOptions({
  global: {
    useUTC: false
  }
});

function requestData() {
  $.ajax({
    url: '/1/lb/stats',
    success: function(point) {
      console.log(point)
      var jason = JSON.parse(point);

      //var series = chart.series[0],
      //shift = series.data.length > 20;

      if (!jason["stats"] || jason["stats"].length == 0) {
        // XXX (reed): using server timestamps for real data this can drift easily
        // XXX (reed): uh how to insert empty data point w/o node data? enum all series names?
        //series.addPoint([(new Date()).getTime(), 0], true, shift);
        //series: [{
        //name: 'Random data',
        //data: []
        //}]
        return
      }

      for (var i = 0; i < jason["stats"].length; i++) {
        stat = jason["stats"][i];
        var node = stat["node"];

        console.log("before", seriesMapper[node])
        if (seriesMapper[node] == null) {
          console.log("yodawg")
          chart.addSeries({name: node, data: []})
          seriesMapper[node] = chart.series.length - 1
          chart.redraw();
        }

        console.log("done", seriesMapper[node])
        series = chart.series[seriesMapper[node]]
        //series = chart.series[0]
        // XXX (reed): hack
        shift = series.data.length > 20 && i == jason["stats"].length + 1;


        timestamp = Date.parse(stat["timestamp"]);
        console.log(series.data.length, timestamp, stat["tp"])
        series.addPoint([timestamp, stat["tp"]], true, shift);
        //series.addPoint({
        //name: node,
        //data: {x: timestamp, y: stat["tp"]}
        //}, true, shift);
      }

      // call it again after one second
      // XXX (reed): this won't work cuz if the endpoint fails then we won't ask for more datas
      setTimeout(requestData, 1000);
    },
    cache: false
  });
}

$(document).ready(function() {
  chart = new Highcharts.Chart({
    chart: {
      renderTo: 'container',
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
        text: 'Value',
        margin: 80
      }
    },
    series: []
  });
});
