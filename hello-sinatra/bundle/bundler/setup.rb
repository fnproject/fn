require 'rbconfig'
# ruby 1.8.7 doesn't define RUBY_ENGINE
ruby_engine = defined?(RUBY_ENGINE) ? RUBY_ENGINE : 'ruby'
ruby_version = RbConfig::CONFIG["ruby_version"]
path = File.expand_path('..', __FILE__)
$:.unshift "#{path}/../#{ruby_engine}/#{ruby_version}/gems/rack-1.6.4/lib"
$:.unshift "#{path}/../#{ruby_engine}/#{ruby_version}/gems/rack-protection-1.5.3/lib"
$:.unshift "#{path}/../#{ruby_engine}/#{ruby_version}/gems/tilt-2.0.1/lib"
$:.unshift "#{path}/../#{ruby_engine}/#{ruby_version}/gems/sinatra-1.4.6/lib"
