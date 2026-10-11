// The hyphenation patterns the site carries: the subset of internal/typesetting/hyphen/patterns
// (hyph-en-us, hyph-fr, hyph-de-1996, from hyph-utf8) that matches the example words of the
// chapter "Breaking lines" and the paragraph of its line breaker. Every pattern that matches those
// words is here, so they hyphenate exactly as on the reader; another word may miss patterns.
// Generated from the pattern files; do not edit by hand.

export const patternSubsets = {
  en: {
    total: 4938,
    patterns: `.ch4 .ev2 .in1 .in2k .li2n .of5te .out3 .si2 .st4 .ta4 .th2 1co 1fi 1fo 1ga 1go 1gr 1l4ine
      1ly 1ma 1na 1pa 1phy 1so 1su 1ta 1tio 1to 1ty 1wo2 2a2r 2ch 2d1s2 2ft 2in. 2ine 2io 2ith
      2lf 2ne. 2p1t 2s1in 2t1ed 2t1in 2ui2 2ze 3raphy 4and 4as. 4be. 4ch. 4graphy 4h1m 4ink
      4k1s2 4l1g4 4m1n 4m1p 4nk2 4oth 4p1p 4ph. 4t1s2 4t3t2 5fon 5graph. 5po4g 5pute 5siz a2d
      a2n a4i4n ad5er. ai2 an5ot ano4 anoth5 ays4 b2l2 b4le. ck1 e1f e1s2e ead1 el2f ev1er f4l2
      fon4t gl2 hap5t he2n hen5at hena4 hy3ph kes4 l5umn. lgo3 n1t n2at n5eve ne4v ne4w o2n ou2
      p2pe p2te p3pen parag6ra4 pr2 pri4s pu2t put3er qu2 r1f r1p r1r4 r2is ry3t s2h s3ing su2r
      th2e type3 w5abl x1t2 y3po y3thin`,
  },
  fr: {
    total: 1212,
    patterns: `.o4 1b2l 1bi 1di 1hy 1hè 1hé 1li 1na 1p2h 1q 1t2h 1te 1ti 4que.`,
  },
  de: {
    total: 36709,
    patterns: `.silbe4 1b 1d 1fa 1g 1re 1sc 1si 1t 2aud 2ch2re 2cht 2f1f 2fs 2hr 2ht 2if 2lb 2mp 2n1n 2ng
      2nt 2o1na 2pfs2 2rt 2ts 2ud 3don2a 3n2ung 3silb 5rechts 5trennu a2u am2pf b2en chr2
      don4au3 fff2 fsch2 htsch4 n1au nt2r nu2n r2eib s2chi s2il t2re tr2 u3d2a un1`,
  },
} as const;
