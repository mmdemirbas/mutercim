FROM texlive/texlive:latest-minimal

RUN tlmgr install \
    polyglossia \
    bidi \
    xetex \
    fontspec \
    arabxetex \
    fancyhdr \
    geometry \
    hyperref \
    bookmark \
    enumitem \
    titlesec \
    amiri

RUN apt-get update && apt-get install -y \
    fonts-amiri \
    fonts-hosny-amiri \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /data
ENTRYPOINT ["xelatex", "-interaction=nonstopmode"]
