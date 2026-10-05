fold-left(
    (1 to 3) ! ('sheet' || . || '.xsl'),
    .,
    function($a, $s) {
      transform(
        map {
          'source-node' : $a,
          'stylesheet-location' : $s,
          'delivery-format' : 'document'
        }
      )?output
    }
)