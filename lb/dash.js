
jQuery.noConflict();
var example = 'dynamic-update', 
  theme = 'default';
(function($){ // encapsulate jQuery

  Highcharts.setOptions({
    global: {
      useUTC: false
    }
  });

  // Create the chart
  Highcharts.stockChart('container', {
    chart: {
      events: {
        load: function () {
          setInterval(function () {
            var xmlhttp = new XMLHttpRequest();
            var url = "/1/lb/stats";

            xmlhttp.onreadystatechange = function() {
              if (this.readyState == 4 && this.status == 200) {
                var jason = JSON.parse(this.responseText);

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

                  var node = jason["stats"]
                  var series = chart.get(node)
                  if (!series) {
                    this.addSeries({name: node, data: []})
                    series = chart.get(node) // XXX (reed): meh
                  }

                  shift = series.data.length > 20;


                  stat = jason["stats"][i];
                  timestamp = Date.parse(stat["timestamp"]);
                  // series.addPoint([timestamp, stat["tp"]], true, shift);
                  console.log(stat["node"]);
                  series.addPoint({
                    name: stat["node"],
                    data: {x: timestamp, y: stat["tp"]}
                  }, true, shift);
                }
              }
            };
            xmlhttp.open("GET", url, true);
            xmlhttp.send();
          }, 1000);
        }
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
      inputEnabled: false,
      selected: 0
    },

    title: {
      text: 'lb data'
    },

    exporting: {
      enabled: false
    },

    series: []
  });

})(jQuery);
