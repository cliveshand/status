.PHONY: all list clean %.fetch lint %.lint build publish serve
.PRECIOUS: %/

lc	= $(shell echo $1 | tr '[:upper:]' '[:lower:]')
uc	= $(shell echo $1 | tr '[:lower:]' '[:upper:]')

MAKECMDGOALS	?= clean build
LINT_TARGETS	?= shell yaml hugo
SASS_VER		?= 1.55.0
SASS_PKG		?= sass_embedded
SASS_DIR		?= $(SASS_PKG)
SASS_BIN		?= dart-sass-embedded
SASS_URL		?= https://github.com/sass/$(SASS_BIN)/releases/download/$(SASS_VER)/$(SASS_PKG)-$(SASS_VER)-linux-x64.tar.gz
HUGO_BIN		?= hugo
HUGO_ENV		?= production
TINA_ENV		?= $(HUGO_ENV)
S3_BUCKET		?= advana-pet-status


all:	$(MAKECMDGOALS)

list:
	@LC_ALL=C $(MAKE) -Rrqp | grep -E '^[[:alnum:][:punct:]]*:([[:space:]]|$$)' | sed -e '/^[%/.]/d;s/:\([[:space:]].*\)\?//;/^Makefile$$/d' | sort

%/:
	install -d "$@"

clean:
	rm -rf public resources

lint: $(addsuffix .lint,$(LINT_TARGETS))

shell.lint:
	set

yaml.lint:
	find -name "*.yml" -o -name "*.yaml" -exec python -c "import yaml; print(yaml.dump(yaml.load(open('{}'))))" \;

hugo.lint:
	$(HUGO_BIN) env

build:
	HUGO_ENV=$(HUGO_ENV) $(HUGO_BIN) -v

serve:
	npx netlify-cms-proxy-server &
	HUGO_ENV=$(HUGO_ENV) $(HUGO_BIN) server -v --disableFastRender --renderToDisk
#	tinacms $(TINA_ENV) -c "hugo server -D -v"

publish:
	git push
#	aws s3 sync --delete --acl public-read public "s3://${S3_BUCKET}"
