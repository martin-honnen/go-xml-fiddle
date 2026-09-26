<?xml version="1.0" encoding="utf-8"?>
<xsl:stylesheet xmlns:xsl="http://www.w3.org/1999/XSL/Transform"
  version="3.0"
  xmlns:xs="http://www.w3.org/2001/XMLSchema"
  xmlns:mf="http://example.com/mf"
  exclude-result-prefixes="#all"
  expand-text="yes">

  <xsl:output method="xml" indent="yes"/>

  <xsl:mode on-no-match="shallow-copy"/>
  
  <xsl:function name="mf:group" as="node()*">
    <xsl:param name="elements" as="element()*"/>
    <xsl:sequence select="mf:group($elements, 1)"/>
  </xsl:function>
  
  <xsl:function name="mf:group" as="node()*">
    <xsl:param name="elements" as="element()*"/>
    <xsl:param name="level" as="xs:integer"/>
    <xsl:for-each-group select="$elements" group-starting-with="*[local-name() = 'h' || $level]">
      <xsl:choose>
        <xsl:when test="self::*[local-name() = 'h' || $level]">
          <concept id="{generate-id()}">
            <title>
              <xsl:apply-templates/>
            </title>
            <conbody>
              <xsl:sequence select="mf:group(current-group() except ., $level + 1)"/>
            </conbody>
          </concept>
        </xsl:when>
        <xsl:otherwise>
          <xsl:for-each-group select="current-group()" group-adjacent=". instance of element(li)">
            <xsl:choose>
              <xsl:when test="current-grouping-key()">
                <ul>
                  <xsl:apply-templates select="current-group()"/>
                </ul>
              </xsl:when>
              <xsl:otherwise>
                <xsl:apply-templates select="current-group()"/>
              </xsl:otherwise>
            </xsl:choose>
          </xsl:for-each-group>
        </xsl:otherwise>
      </xsl:choose>
    </xsl:for-each-group>
  </xsl:function>
  
  <xsl:template match="doc">
    <xsl:sequence select="mf:group(*)"/>
  </xsl:template>

  <xsl:template match="/" name="xsl:initial-template">
    <xsl:copy>
      <xsl:apply-templates/>
      <xsl:comment>Run with {system-property('xsl:product-name')} {system-property('xsl:product-version')} at {current-dateTime()}</xsl:comment>
    </xsl:copy>
  </xsl:template>

</xsl:stylesheet>
