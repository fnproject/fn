import sys
sys.path.append("packages")
import os
import json

sys.stderr.write("Starting Python Function\n")

name = "World"

try:
  if not os.isatty(sys.stdin.fileno()):
    obj = json.loads(sys.stdin.read())
    if obj["name"] != "":
      name = obj["name"]
except:
  pass

print "Hello", name, "!"
