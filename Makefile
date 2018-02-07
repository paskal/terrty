# build image with latest jsonresume-theme-kendall version
build-image-resumejson:
	docker build -t paskal/jsonresume cv

# resume.json generation, html and pdf
resume:
	touch $(PWD)/blog/public/cv/verhoturov.html
	docker run -it --rm --name build-resume \
		--mount type=bind,source=$(PWD)/blog/public/cv/verhoturov.html,target=/data/verhoturov.html \
		--mount type=bind,source=$(PWD)/cv/resume.json,target=/data/resume.json \
		paskal/jsonresume \
		export --theme kendall verhoturov.html
	docker run --rm --name build-pdf \
		--mount type=bind,source=$(PWD)/blog/public/cv,target=/tmp/html-to-pdf \
		--privileged pink33n/html-to-pdf \
		--url https://terrty.net/cv/verhoturov.html \
		--pdf verhoturov.pdf

# blog generation
.PHONY: blog # to get rid of "target is up to date"
blog:
	cd blog && hugo