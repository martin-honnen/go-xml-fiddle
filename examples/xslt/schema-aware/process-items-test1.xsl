<xsl:stylesheet
  xmlns:xsl="http://www.w3.org/1999/XSL/Transform"
  xmlns:xs="http://www.w3.org/2001/XMLSchema"
  exclude-result-prefixes="#all"
  expand-text="yes"
  version="3.0">

  <xsl:import-schema schema-location="schema-sample1.xsd"/>

  <xsl:mode on-no-match="shallow-copy" default-validation="preserve"/>

  <xsl:output indent="yes"/>

  <xsl:template match="item/value">
    <xsl:comment>data() instance of xs:decimal: {data() instance of xs:decimal}</xsl:comment>
    <xsl:next-match/>     
  </xsl:template>

</xsl:stylesheet>