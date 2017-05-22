
var example = 'dynamic-update', 
  theme = 'default';

var chart; // global

function requestData() {
  $.ajax({
    url: '/1/lb/stats',
    success: function(point) {
      console.log(point)
      var jason = JSON.parse(point);
      console.log(jason)

      var series = chart.series[0],
        shift = series.data.length > 20;

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
        // set up the updating of the chart each second

        //var node = jason["stats"]
        //var series = chart.get(node)
        //if (!series) {
          //chart.addSeries({name: node, data: []})
          //series = chart.get(node) // XXX (reed): meh
        //}

        //shift = series.data.length > 20;

        stat = jason["stats"][i];
        timestamp = Date.parse(stat["timestamp"]);
         series.addPoint([timestamp, stat["tp"]], true, shift);
        //series.addPoint({
          //name: stat["node"],
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
      defaultSeriesType: 'spline',
      events: {
        load: requestData
      }
    },
    title: {
      text: 'Live random data'
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
    series: [{
      name: 'Random data',
      data: []
    }]
  });
});
