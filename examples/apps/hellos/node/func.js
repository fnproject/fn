fs = require('fs');

name = "do you speak node?";
try {
	obj = JSON.parse(fs.readFileSync('/dev/stdin').toString())
	if (obj.name != "") {
		name = obj.name
	}
} catch(e) {}
console.log("Hello, " + name);
