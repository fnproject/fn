# Copyright 2016 Iron.io
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

require 'open3'

class ExecError < StandardError 
  attr_accessor :exit_status, :last_line
  
  def initialize(exit_status, last_line)
    super("Error on cmd. #{exit_status}")
    self.exit_status = exit_status
    self.last_line = last_line
  end
end

def stream_exec(cmd)
  puts "Executing cmd: #{cmd}"
  exit_status = nil
  last_line = ""
  Open3.popen2e(cmd) do |stdin, stdout_stderr, wait_thread|
    Thread.new do
      stdout_stderr.each {|l| 
        puts l
        # Save last line for error checking
        last_line = l
      }
    end

    # stdin.puts 'ls'
    # stdin.close

    exit_status = wait_thread.value
    raise ExecError.new(exit_status, last_line) if exit_status.exitstatus != 0
  end
  return exit_status
end

def exec(cmd)
  puts "Executing: #{cmd}"
  puts `#{cmd}`
end
