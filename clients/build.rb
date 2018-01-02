require 'yaml'
require 'open-uri'
require 'http'
require 'fileutils'
require 'openssl'

require_relative 'utils.rb'

swaggerUrl = "https://raw.githubusercontent.com/fnproject/fn/master/docs/swagger.yml"
spec = YAML.load(open(swaggerUrl))
version = spec['info']['version']
puts "VERSION: #{version}"

# Can pass in a particular language to only do that one
only = ARGV[0]
puts "only building: #{only}" if only

# Keep getting cert errors??  Had to do this to work around it:
ctx = OpenSSL::SSL::SSLContext.new
ctx.verify_mode = OpenSSL::SSL::VERIFY_NONE

def clone(lang)
  Dir.chdir 'tmp'
  ldir = "fn_#{lang}"
  if !Dir.exist? ldir
    cmd = "git clone https://github.com/fnproject/#{ldir}"
    stream_exec(cmd)
  else
    Dir.chdir ldir
    cmd = "git pull"
    stream_exec(cmd)
    Dir.chdir '../'
  end
  Dir.chdir '../'
  return "tmp/#{ldir}"
end

FileUtils.mkdir_p 'tmp'
languages = nil
# See all supported langauges here: https://generator.swagger.io/api/gen/clients
if only
  languages = [only]
else
  languages = ['go', 'ruby', 'php', 'python', 'elixir', 'javascript', 'java'] # JSON.parse(HTTP.get("https://generator.swagger.io/api/gen/clients", ssl_context: ctx).body)
end
languages.each do |l|
  puts "\nGenerating client for #{l}..."
  lshort = l
  glob_pattern = ["**", "*"]
  copy_dir = "."
  options = {}
  skip_files = []
  deploy = []
  clone_dir = ""
  case l
  when 'go'
    clone_dir = clone(lshort)
    glob_pattern = ['functions', "**", "*.go"]
    copy_dir = "."
    options['packageName'] = 'functions'
    options['packageVersion'] = version
  when 'ruby'
    clone_dir = clone(l)
    fruby = "fn_ruby"
    gem_name = "fn_ruby"
    glob_pattern = ["**", "*.rb"] # just rb files
    skip_files = [] # ["#{gem_name}.gemspec"]
    deploy = ["gem build #{gem_name}.gemspec", "gem push #{gem_name}-#{version}.gem"]
    options['gemName'] = gem_name
    options['moduleName'] = "Fn"
    options['gemVersion'] = version
    options['gemHomepage'] = "https://github.com/fnproject/#{fruby}"
    options['gemSummary'] = 'Ruby gem for Fn Project'
    options['gemDescription'] = 'Ruby gem for Fn Project.'
    options['gemAuthorEmail'] = 'treeder@gmail.com'
  when 'javascript'
    lshort = 'js'
    # copy_dir = "javascript-client/."
    clone_dir = clone(lshort)
    options['projectName'] = "fn_js"
    deploy << "npm publish"
  else
    clone_dir = clone(l)
  end
  p options
  lv = "#{lshort}-#{version}"
  destdir = "tmp/fn_#{lshort}"
  if l == 'go'
    # This is using https://goswagger.io/ instead
    stream_exec "docker run --rm -v ${PWD}/#{clone_dir}:/go/src/github.com/fnproject/fn_go -v ${GOPATH}/src/github.com/fnproject/fn/docs/swagger.yml:/go/src/github.com/fnproject/fn/swagger.yml -w /go/src/github.com/fnproject/fn_go quay.io/goswagger/swagger generate client -f /go/src/github.com/fnproject/fn/swagger.yml -A fn "
  else
    gen = JSON.parse(HTTP.post("https://generator.swagger.io/api/gen/clients/#{l}",
      json: {
        swaggerUrl: swaggerUrl,
        options: options,
      },
      ssl_context: ctx).body)
    p gen

    zipfile = "tmp/#{lv}.zip"
    stream_exec "curl -o #{zipfile} #{gen['link']} -k"
    stream_exec "unzip -o #{zipfile} -d tmp/#{lv}"

    # delete the skip_files
    skip_files.each do |sf|
      begin
        File.delete("tmp/#{lv}/#{lshort}-client/" + sf)
      rescue => ex
        puts "Error deleting file: #{ex.backtrace}"
      end
    end

    # Copy into clone repos
    fj = File.join(['tmp', lv, "#{l}-client"] + glob_pattern)
    puts "Trying cp", "tmp/#{lv}/#{l}-client/#{copy_dir}", destdir
    FileUtils.cp_r("tmp/#{lv}/#{l}-client/#{copy_dir}", destdir)
    # Write a version file, this ensures there's always a change.
  end
  File.open("#{destdir}/VERSION", 'w') { |file| file.write(version) }


  # Commit and push
  begin
    Dir.chdir(destdir)
    stream_exec "git add ."
    stream_exec "git commit -am \"Updated to api version #{version}\""
    begin
      stream_exec "git tag -a #{version} -m \"Version #{version}\""
    rescue => ex
      puts "WARNING: Tag #{version} already exists."
    end
    stream_exec "git push --follow-tags"
    deploy.each do |d|
      stream_exec d
    end
  rescue ExecError => ex
    puts "Error: #{ex}"
    if ex.last_line.include?("nothing to commit") || ex.last_line.include?("already exists") || ex.last_line.include?("no changes added to commit")
       # ignore this
       puts "Ignoring error"
    else
       raise ex
    end
  end
  Dir.chdir("../../")

end
