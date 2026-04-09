FROM texlive/texlive:TL2024-historic-minimal

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
    amiri \
    etoolbox \
    tools \
    zref \
    auxhook \
    atbegshi \
    atveryend \
    bitset \
    ltxcmds \
    pdftexcmds \
    infwarerr \
    uniquecounter \
    rerunfilecheck

# Register TeX Live fonts with fontconfig so XeTeX can find Amiri etc.
RUN cp "$(find /usr/local/texlive -name texlive-fontconfig.conf | head -1)" \
       /etc/fonts/conf.d/09-texlive-fonts.conf 2>/dev/null; \
    fc-cache -fv > /dev/null 2>&1 || true

# Add TeX Live bin to PATH (architecture-independent)
ENV PATH="$(find /usr/local/texlive -type d -name bin -path '*/bin' | head -1)/*:${PATH}"
RUN for bin in /usr/local/texlive/*/bin/*/xelatex; do ln -sf "$bin" /usr/local/bin/xelatex; done

WORKDIR /data
ENTRYPOINT ["xelatex", "-interaction=nonstopmode"]
