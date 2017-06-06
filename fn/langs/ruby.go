package langs

type RubyLangHelper struct {
	BaseHelper
}

func (lh *RubyLangHelper) BuildFromImage() string {
	return "funcy/ruby:dev"
}

func (lh *RubyLangHelper) RunFromImage() string {
	return "funcy/ruby"
}

func (h *RubyLangHelper) DockerfileBuildCmds() []string {
	r := []string{}
	if exists("Gemfile") {
		r = append(r,
			"ADD Gemfile* /function/",
			"RUN bundle install",
		)
	}
	return r
}

func (h *RubyLangHelper) DockerfileCopyCmds() []string {
	return []string{
		"COPY --from=build-stage /usr/lib/ruby/gems/ /usr/lib/ruby/gems/", // skip this if no Gemfile?  Does it matter?
		"ADD . /function/",
	}
}

func (lh *RubyLangHelper) Entrypoint() string {
	return "ruby func.rb"
}
