# Run with: brew ruby tools/test_homebrew_formula.rb FORMULA DIST VERSION
# Uses Homebrew's real Formula DSL and install helpers, with only the prefix
# redirected into a temporary directory. It does not register a tap, link a keg,
# download anything, install dependencies or change the user's profile.
require "formula"
require "open3"
require "simulate_system"
require "tmpdir"
require "digest"
require "json"

formula_path, archive_directory, version = ARGV
abort "Expected FORMULA DIST VERSION" unless ARGV.length == 3
formula_path = Pathname.new(formula_path).realpath
archive_directory = Pathname.new(archive_directory).realpath
checksums = (archive_directory/"SHA256SUMS-#{version}.txt").read.lines.to_h do |line|
  checksum, filename = line.split
  [filename, checksum]
end
results = []

{ macos: "darwin", linux: "linux" }.each do |os, system|
  { arm: "arm64", intel: "amd64" }.each do |arch, machine|
    Homebrew::SimulateSystem.with(os:, arch:) do
      Object.send(:remove_const, :Perch) if Object.const_defined?(:Perch)
      load formula_path
      # Supplying an uninstalled tap avoids consulting the Homebrew core API.
      formula = Perch.new("perch", formula_path, :stable, tap: Tap.fetch("darthnithin/tap"))
      filename = "perch-#{version}-#{system}-#{machine}.tar.gz"
      archive = archive_directory/filename
      raise "Wrong platform URL" unless formula.stable.url.end_with?("/#{version}/#{filename}")
      raise "Wrong platform checksum" unless formula.stable.checksum.to_s == checksums.fetch(filename)
      raise "Damaged archive" unless Digest::SHA256.file(archive).hexdigest == checksums.fetch(filename)
      raise "Unexpected compiler/dependency" unless formula.deps.empty?

      Dir.mktmpdir("perch-brew-test-") do |directory|
        root = Pathname.new(directory)
        prefix = root/"installed"
        stage = root/"stage"
        stage.mkpath
        # Archives were independently validated by test_release_archives.py.
        raise "Cannot extract" unless system("tar", "-xzf", archive.to_s, "-C", stage.to_s)
        binary_hash = Digest::SHA256.file(stage/"perch").hexdigest
        formula.define_singleton_method(:prefix) { |*_args| prefix }
        formula.define_singleton_method(:opt_prefix) { prefix }
        Dir.chdir(stage) { formula.install }
        raise "Binary changed" unless Digest::SHA256.file(prefix/"bin/perch").hexdigest == binary_hash
        raise "Missing executable mode" unless (prefix/"bin/perch").executable?
        %w[README.md docs/GUIDE.md docs/media/workspace.gif
           docs/terminal/Perch-Ioskeley.terminal assets/bird/terminal-running-strip.png
           THIRD_PARTY_NOTICES.md third_party/manifest.json].each do |path|
          raise "Missing bundled #{path}" unless (formula.doc/path).file?
        end
        raise "Wrong profile instructions" unless formula.caveats.include?((formula.doc/"docs/terminal/Perch-Ioskeley.terminal").to_s)
        # Smoke-test the installed executable, without Homebrew's optional test
        # gems. This is not `brew test`. Intel on Apple Silicon uses Rosetta.
        can_run = system == "darwin" && RUBY_PLATFORM.include?("darwin")
        if can_run
          executable = (prefix/"bin/perch").to_s
          output, status = Open3.capture2(executable, "version")
          raise "Wrong installed version" unless status.success? && output.strip == "perch #{version}"
          output, status = Open3.capture2(executable, "jobs", "--demo", "--no-config", "--json")
          raise "Demo command failed" unless status.success?
          result = JSON.parse(output)
          raise "Wrong demo result" unless result.fetch("source") == "demo" && !result.fetch("data").empty?
        end
        results << { target: "#{system}-#{machine}", version: version,
                     archive_sha256: checksums.fetch(filename), binary_sha256: binary_hash,
                     selection: "passed", install: "passed",
                     runtime_smoke: can_run ? "passed (Mac host)" : "not executed here" }
      end
    end
  end
end
puts JSON.pretty_generate(results)
