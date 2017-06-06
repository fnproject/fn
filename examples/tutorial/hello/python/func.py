import sys
import os
import json

sys.stderr.write("Starting Python Function\n")

name = "World"

try:
  if not os.isatty(sys.stdin.fileno()):
    try:
      obj = json.loads(sys.stdin.read())
      if obj["name"] != "":
        name = obj["name"]
    except ValueError:
      # ignore it
      sys.stderr.write("no input, but that's ok\n")
except:
  pass

print "Hello", name, "!"
