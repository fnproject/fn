'use strict';

var fs = require('fs');

var oldlog = console.log
console.log = console.error

// Some notes on the semantics of the succeed(), fail() and done() methods.
// Tests are the source of truth!
// First call wins in terms of deciding the result of the function. BUT,
// subsequent calls also log. Further, code execution does not stop, even where
// for done(), the docs say that the "function terminates". It seems though
// that further cycles of the event loop do not run. For example:
// index.handler = function(event, context) {
//   context.fail("FAIL")
//   process.nextTick(function() {
//     console.log("This does not get logged")
//   })
//   console.log("This does get logged")
// }
// on the other hand:
// index.handler = function(event, context) {
//   process.nextTick(function() {
//     console.log("This also gets logged")
//     context.fail("FAIL")
//   })
//   console.log("This does get logged")
// }
//
// The same is true for context.succeed() and done() captures the semantics of
// both. It seems this is implemented simply by having process.nextTick() cause
// process.exit() or similar, because the following:
// exports.handler = function(event, context) {
//     process.nextTick(function() {console.log("This gets logged")})
//     process.nextTick(function() {console.log("This also gets logged")})
//     context.succeed("END")
//     process.nextTick(function() {console.log("This does not get logged")})
// };
//
// So the context object needs to have some sort of hidden boolean that is only
// flipped once, by the first call, and dictates the behavior on the next tick.
//
// In addition, the response behaviour depends on the invocation type. If we
// are to only support the async type, succeed() must return a 202 response
// code, not sure how to do this.
//
// Only the first 256kb, followed by a truncation message, should be logged.
//
// Also, the error log is always in a json literal
// { "errorMessage": "<message>" }
var Context = function() {
  var concluded = false;

  var contextSelf = this;

  // The succeed, fail and done functions are public, but access a private
  // member (concluded). Hence this ugly nested definition.
  this.succeed = function(result) {
    if (concluded) {
      return
    }

    // We have to process the result before we can conclude, because otherwise
    // we have to fail. This means NO EARLY RETURNS from this function without
    // review!
    if (result === undefined) {
      result = null
    }

    var failed = false;
    try {
      // Output result to log
      oldlog(JSON.stringify(result));
    } catch(e) {
      // Set X-Amz-Function-Error: Unhandled header
      console.log("Unable to stringify body as json: " + e);
      failed = true;
    }

    // FIXME(nikhil): Return 202 or 200 based on invocation type and set response
    // to result. Should probably be handled externally by the runner/swapi.

    // OK, everything good.
    concluded = true;
    process.nextTick(function() { process.exit(failed ? 1 : 0) })
  }

  this.fail = function(error) {
    if (concluded) {
      return
    }

    concluded = true
    process.nextTick(function() { process.exit(1) })

    if (error === undefined) {
      error = null
    }

    // FIXME(nikhil): Truncated log of error, plus non-truncated response body
    var errstr = "fail() called with argument but a problem was encountered while converting it to a to string";

    // The semantics of fail() are weird. If the error is something that can be
    // converted to a string, the log output wraps the string in a JSON literal
    // with key "errorMessage". If toString() fails, then the output is only
    // the error string.
    try {
      if (error === null) {
        errstr = null
      } else {
        errstr = error.toString()
      }
      oldlog(JSON.stringify({"errorMessage": errstr }))
    } catch(e) {
      // Set X-Amz-Function-Error: Unhandled header
      oldlog(errstr)
    }
  }

  this.done = function() {
    var error = arguments[0];
    var result = arguments[1];
    if (error) {
      contextSelf.fail(error)
    } else {
      contextSelf.succeed(result)
    }
  }

  var plannedEnd = Date.now() + (getTimeoutInSeconds() * 1000);
  this.getRemainingTimeInMillis = function() {
    return Math.max(plannedEnd - Date.now(), 0);
  }
}

function getTimeoutInSeconds() {
  var t = parseInt(getEnv("TASK_TIMEOUT"));
  if (Number.isNaN(t)) {
    return 3600;
  }

  return t;
}

var getEnv = function(name) {
  return process.env[name] || "";
}

var makeCtx = function() {
  var fnname = getEnv("AWS_LAMBDA_FUNCTION_NAME");
  // FIXME(nikhil): Generate UUID.
  var taskID = getEnv("TASK_ID");

  var mem = getEnv("TASK_MAXRAM").toLowerCase();
  var bytes = 300 * 1024 * 1024;

  var scale = { 'b': 1, 'k': 1024, 'm': 1024*1024, 'g': 1024*1024*1024 };
  // We don't bother validating too much, if the last character is not a number
  // and not in the scale table we just return a default value.
  // We use slice instead of indexing so that we always get an empty string,
  // instead of undefined.
  if (mem.slice(-1).match(/[0-9]/)) {
    var a = parseInt(mem);
    if (!Number.isNaN(a)) {
      bytes = a;
    }
  } else {
    var rem = parseInt(mem.slice(0, -1));
    if (!Number.isNaN(rem)) {
      var multiplier = scale[mem.slice(-1)];
      if (multiplier) {
        bytes = rem * multiplier
      }
    }
  }

  var memoryMB = bytes / (1024 * 1024);

  var ctx = new Context();
  Object.defineProperties(ctx, {
    "functionName": {
      value: fnname,
      enumerable: true,
    },
    "functionVersion": {
      value: "$LATEST",
      enumerable: true,
    },
    "invokedFunctionArn": {
      // FIXME(nikhil): Should be filled in.
      value: "",
      enumerable: true,
    },
    "memoryLimitInMB": {
      // Sigh, yes it is a string.
      value: ""+memoryMB,
      enumerable: true,
    },
    "awsRequestId": {
      value: taskID,
      enumerable: true,
    },
    "logGroupName": {
      // FIXME(nikhil): Should be filled in.
      value: "",
      enumerable: true,
    },
    "logStreamName": {
      // FIXME(nikhil): Should be filled in.
      value: "",
      enumerable: true,
    },
    "identity": {
      // FIXME(nikhil): Should be filled in.
      value: null,
      enumerable: true,
    },
    "clientContext": {
      // FIXME(nikhil): Should be filled in.
      value: null,
      enumerable: true,
    },
  });

  return ctx;
}

var setEnvFromHeader = function () {
  var headerPrefix = "CONFIG_";
  var newEnvVars = {};
  for (var key in process.env) {
    if (key.indexOf(headerPrefix) == 0) {
      newEnvVars[key.slice(headerPrefix.length)] = process.env[key];
    }
  }

  for (var key in newEnvVars) {
    process.env[key] = newEnvVars[key];
  }
}


function run() {
  setEnvFromHeader();
  // FIXME(nikhil): Check for file existence and allow non-payload.
  var path = process.env["PAYLOAD_FILE"];
  var stream = process.stdin;
  if (path) {
    try {
      stream = fs.createReadStream(path);
    } catch(e) {
      console.error("bootstrap: Error opening payload file", e)
      process.exit(1);
    }
  }

  var input = "";
  stream.setEncoding('utf8');
  stream.on('data', function(chunk) {
    input += chunk;
  });

  stream.on('error', function(err) {
    console.error("bootstrap: Error reading payload stream", err);
    process.exit(1);
  });

  stream.on('end', function() {
    var payload = {}
    try {
      if (input.length > 0) {
        payload = JSON.parse(input);
      }
    } catch(e) {
      console.error("bootstrap: Error parsing JSON", e);
      process.exit(1);
    }

    if (process.argv.length > 2) {
      var handler = process.argv[2];
      var parts = handler.split('.');
      // FIXME(nikhil): Error checking.
      var script = parts[0];
      var entry = parts[1];
      var started = false;
      try {
        var mod = require('./'+script);
        var func = mod[entry];
        if (func === undefined) {
          oldlog("Handler '" + entry + "' missing on module '" + script + "'");
          return;
        }

        if (typeof func !== 'function') {
          throw "TypeError: " + (typeof func) + " is not a function";
        }
        started = true;
        var cback 
        // RUN THE FUNCTION:
        mod[entry](payload, makeCtx(), functionCallback)
      } catch(e) {
        if (typeof e === 'string') {
          oldlog(e)
        } else {
          oldlog(e.message)
        }
        if (!started) {
          oldlog("Process exited before completing request\n")
        }
      }
    } else {
      console.error("bootstrap: No script specified")
      process.exit(1);
    }
  })
}

function functionCallback(err, result) {
    if (err != null) {
        // then user returned error and we should respond with error
        // http://docs.aws.amazon.com/lambda/latest/dg/nodejs-prog-mode-exceptions.html
        oldlog(JSON.stringify({"errorMessage": errstr }))
        return
    }
    if (result != null) {
        oldlog(JSON.stringify(result))
    }
}

run()