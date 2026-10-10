<?xml version="1.0" encoding="UTF-8"?>
<!--
  ass-tags.xsl — presentation layer for docs/ass-tags.xml.

  Data lives only in the XML; this file holds no behavior claims. Referenced by
  <?xml-stylesheet?> in ass-tags.xml. Browsers will not fetch this stylesheet
  over file:// (opaque origins block it), so read the matrix via the HTML
  export. Regenerate after editing the XML, from the repo root:

    $x = New-Object System.Xml.Xsl.XslCompiledTransform
    $x.Load('docs/ass-tags.xsl'); $x.Transform('docs/ass-tags.xml', 'docs/ass-tags.html')

  docs/ass-tags.html is gitignored: the XML is the source, the HTML is a view.
-->
<xsl:stylesheet version="1.0"
                xmlns:xsl="http://www.w3.org/1999/XSL/Transform"
                xmlns="http://www.w3.org/1999/xhtml">

  <xsl:output method="html" indent="yes" omit-xml-declaration="yes"/>

  <xsl:template match="/">
    <html>
      <head>
        <title>ASS override tag behavior matrix</title>
        <style type="text/css">
          body { font: 14px/1.55 system-ui, sans-serif; margin: 2rem auto; max-width: 74rem; padding: 0 1rem; color: #1b1b1b; }
          h1 { font-size: 1.5rem; margin-bottom: .25rem; }
          h2 { font-size: 1.1rem; margin-top: 2.5rem; border-bottom: 1px solid #ddd; padding-bottom: .25rem; }
          h3 { font-size: 1rem; margin: 1.6rem 0 .35rem; font-family: ui-monospace, Consolas, monospace; }
          table { border-collapse: collapse; width: 100%; margin: .5rem 0 1rem; font-size: .9rem; }
          th, td { border: 1px solid #e2e2e2; padding: .35rem .5rem; vertical-align: top; text-align: left; }
          th { background: #f6f6f7; font-weight: 600; }
          code, .name { font-family: ui-monospace, Consolas, monospace; }
          .badge { font-size: .75rem; font-weight: 700; border-radius: 3px; padding: .05rem .3rem; margin-left: .4rem; vertical-align: middle; }
          .V { background: #dcf5e3; color: #14562c; }
          .S { background: #fdf0d5; color: #6b4a10; }
          .diff { background: #fff4f4; }
          .same { color: #777; }
          .na { color: #999; }
          .note { color: #444; }
          .cite { font-family: ui-monospace, Consolas, monospace; font-size: .78rem; color: #666; }
          .desc { margin: .3rem 0 .6rem; color: #333; }
          .evidence { display: block; margin-top: .2rem; font-size: .75rem; }
          .evidence.verified { color: #14562c; }
          .evidence.inferred { color: #6b4a10; }
          .evidence.unchecked { color: #777; }
          table.params { font-size: .85rem; }
          table.params td[rowspan] { background: #fbfbfc; }
          .legend { background: #fafafa; border: 1px solid #e5e5e5; padding: .6rem .8rem; font-size: .85rem; }
          @media (prefers-color-scheme: dark) {
            body { color: #e6e6e6; background: #17181a; }
            th { background: #232426; }
            th, td { border-color: #34363a; }
            h2 { border-color: #34363a; }
            .legend { background: #1d1e20; border-color: #34363a; }
            .V { background: #17401f; color: #a8e6ba; }
            .S { background: #453413; color: #f2d59b; }
            .diff { background: #3a1f1f; }
            .same, .cite, .note, .desc { color: #b9b9b9; }
            .evidence.verified { color: #a8e6ba; }
            .evidence.inferred { color: #f2d59b; }
            .evidence.unchecked { color: #b9b9b9; }
            table.params td[rowspan] { background: #202124; }
          }
        </style>
      </head>
      <body>
        <xsl:apply-templates select="matrix"/>
      </body>
    </html>
  </xsl:template>

  <xsl:template match="matrix">
    <h1>ASS override tag behavior matrix</h1>
    <p class="cite">
      <xsl:for-each select="meta/pin">
        <xsl:value-of select="@renderer"/> <xsl:text> </xsl:text>
        <code><xsl:value-of select="@commit"/></code>
        <xsl:if test="position() != last()"><xsl:text> · </xsl:text></xsl:if>
      </xsl:for-each>
    </p>

    <div class="legend">
      <strong>How to read a cell.</strong>
      <code>base</code> is libass. Grey and highlighted results compare reported
      behavior; those colors do not indicate source verification. Each renderer
      cell separately says <strong>source verified</strong>,
      <strong>source-inferred</strong>, or <strong>unchecked</strong>.
      A missing renderer value is an unverified assumption of "same as base";
      a missing scenario row inherits its unverified <strong>default</strong>.
      <span class="badge V">V</span> at least one renderer cell was read at a pinned source;
      <span class="badge S">S</span> assumed or reported but not verified.
      The <code>verified</code> scope identifies exactly which renderers have
      cited source evidence for that row. Source verification alone never
      proves a <code>SafeFix</code>: each requested renderer and build capability
      needs a separate edit-equivalence proof.
    </div>

    <h2>Scenario defaults</h2>
    <table>
      <tr><th>scenario</th><th>default result</th></tr>
      <xsl:for-each select="meta/scenarios/scenario">
        <tr>
          <td class="name"><xsl:value-of select="@id"/></td>
          <td><xsl:value-of select="@default"/></td>
        </tr>
      </xsl:for-each>
    </table>

    <h2>Result vocabulary</h2>
    <p class="cite">
      <xsl:for-each select="meta/vocab/result">
        <code><xsl:value-of select="@name"/></code>
        <xsl:if test="position() != last()"><xsl:text>, </xsl:text></xsl:if>
      </xsl:for-each>
    </p>

    <h2>Parameter kinds</h2>
    <table>
      <tr><th>kind</th><th>meaning</th></tr>
      <xsl:for-each select="meta/kinds/kind">
        <tr>
          <td class="name"><xsl:value-of select="@id"/></td>
          <td><xsl:value-of select="@desc"/></td>
        </tr>
      </xsl:for-each>
    </table>

    <h2>Name matching per renderer</h2>
    <p class="desc"><xsl:value-of select="normalize-space(meta/matching)"/></p>

    <h2>Tags</h2>
    <xsl:apply-templates select="tag"/>
    <xsl:apply-templates select="taggroup"/>
  </xsl:template>

  <xsl:template match="tag">
    <h3>
      <xsl:text>\</xsl:text><xsl:value-of select="@name"/>
      <span class="badge {@status}"><xsl:value-of select="@status"/></span>
    </h3>
    <xsl:for-each select="text()">
      <xsl:if test="normalize-space(.) != ''">
        <p class="desc"><xsl:value-of select="normalize-space(.)"/></p>
      </xsl:if>
    </xsl:for-each>
    <xsl:if test="params">
      <table class="params">
        <tr>
          <th>signature</th><th>#</th><th>parameter</th><th>kind</th><th>legal range</th>
          <th>status / source</th>
        </tr>
        <xsl:for-each select="params/sig">
          <xsl:variable name="siglabel">
            <xsl:text>\</xsl:text><xsl:value-of select="../../@name"/>
            <xsl:choose>
              <xsl:when test="@form = 'bare'">
                <xsl:if test="@n != '0' and @n != '1'"><xsl:text>…</xsl:text></xsl:if>
              </xsl:when>
              <xsl:when test="@form = 'paren'">
                <xsl:text>(</xsl:text>
                <xsl:if test="@n != '0'"><xsl:text>…</xsl:text></xsl:if>
                <xsl:text>)</xsl:text>
              </xsl:when>
              <xsl:otherwise>
                <xsl:text>(…)</xsl:text>
                <xsl:if test="@n != '1'"><xsl:text> / \</xsl:text><xsl:value-of select="../../@name"/><xsl:text>…</xsl:text></xsl:if>
              </xsl:otherwise>
            </xsl:choose>
          </xsl:variable>
          <xsl:variable name="rowspan" select="count(p)"/>
          <xsl:for-each select="p">
            <tr>
              <xsl:if test="position() = 1">
                <td rowspan="{$rowspan}" class="name">
                  <xsl:value-of select="$siglabel"/>
                  <br/>
                  <span class="cite">
                    <xsl:text>n=</xsl:text><xsl:value-of select="../@n"/>
                    <xsl:text> </xsl:text><xsl:value-of select="../@form"/>
                    <xsl:if test="../@renderer != 'libass xy vsm'">
                      <xsl:text> · only </xsl:text><xsl:value-of select="../@renderer"/>
                    </xsl:if>
                    <xsl:if test="../@requires">
                      <xsl:text> · requires </xsl:text><xsl:value-of select="../@requires"/>
                    </xsl:if>
                  </span>
                </td>
              </xsl:if>
              <td><xsl:value-of select="@i"/></td>
              <td class="name"><xsl:value-of select="@name"/></td>
              <td><xsl:value-of select="@kind"/></td>
              <td><xsl:value-of select="@range"/></td>
              <xsl:if test="position() = 1">
                <td rowspan="{$rowspan}">
                  <span class="badge {../@status}">
                    <xsl:value-of select="../@status"/>
                  </span>
                  <div class="cite">
                    <xsl:text>verified: </xsl:text>
                    <xsl:choose>
                      <xsl:when test="../@verified"><xsl:value-of select="../@verified"/></xsl:when>
                      <xsl:otherwise>none</xsl:otherwise>
                    </xsl:choose>
                    <xsl:if test="../@inferred">
                      <xsl:text> · inferred: </xsl:text><xsl:value-of select="../@inferred"/>
                    </xsl:if>
                  </div>
                  <div class="cite"><xsl:value-of select="../@cite"/></div>
                </td>
              </xsl:if>
            </tr>
          </xsl:for-each>
        </xsl:for-each>
      </table>
    </xsl:if>
    <xsl:if test="scen">
      <table>
        <tr>
          <th>scenario</th><th>libass</th><th>xy-VSFilter</th><th>VSFilterMod</th>
          <th>status / note</th><th>source</th>
        </tr>
        <xsl:for-each select="scen">
          <tr>
            <td class="name">
              <xsl:value-of select="@id"/>
              <xsl:if test="@status = 'V'"> <span class="badge V">V</span></xsl:if>
              <xsl:if test="@status = 'S'"> <span class="badge S">S</span></xsl:if>
            </td>
            <xsl:call-template name="scenario-cell">
              <xsl:with-param name="renderer" select="'libass'"/>
              <xsl:with-param name="value" select="@base"/>
              <xsl:with-param name="base" select="@base"/>
            </xsl:call-template>
            <xsl:call-template name="scenario-cell">
              <xsl:with-param name="renderer" select="'xy'"/>
              <xsl:with-param name="value" select="@vsfilter"/>
              <xsl:with-param name="base" select="@base"/>
            </xsl:call-template>
            <xsl:call-template name="scenario-cell">
              <xsl:with-param name="renderer" select="'vsm'"/>
              <xsl:with-param name="value" select="@vsfiltermod"/>
              <xsl:with-param name="base" select="@base"/>
            </xsl:call-template>
            <td class="note"><xsl:value-of select="normalize-space(.)"/></td>
            <td class="cite"><xsl:value-of select="@cite"/></td>
          </tr>
        </xsl:for-each>
      </table>
    </xsl:if>
  </xsl:template>

  <!-- A row-level V is never a license to mark all three renderer cells V. -->
  <xsl:template name="scenario-cell">
    <xsl:param name="renderer"/>
    <xsl:param name="value"/>
    <xsl:param name="base"/>
    <xsl:variable name="source-verified"
                  select="@status = 'V' and contains(concat(' ', normalize-space(@verified), ' '), concat(' ', $renderer, ' '))"/>
    <td>
      <xsl:attribute name="class">
        <xsl:choose>
          <xsl:when test="$value = 'unchecked'">na</xsl:when>
          <xsl:when test="$value = '' or $value = $base">same</xsl:when>
          <xsl:otherwise>diff</xsl:otherwise>
        </xsl:choose>
      </xsl:attribute>
      <xsl:choose>
        <xsl:when test="$value = ''">same as base</xsl:when>
        <xsl:otherwise><xsl:value-of select="$value"/></xsl:otherwise>
      </xsl:choose>
      <xsl:choose>
        <xsl:when test="$value = 'unchecked'">
          <span class="evidence unchecked">not source verified</span>
        </xsl:when>
        <xsl:when test="$value = ''">
          <span class="evidence unchecked">inherited (unverified)</span>
        </xsl:when>
        <xsl:when test="$source-verified">
          <span class="evidence verified">source verified</span>
        </xsl:when>
        <xsl:otherwise>
          <span class="evidence inferred">source-inferred</span>
        </xsl:otherwise>
      </xsl:choose>
    </td>
  </xsl:template>

  <xsl:template match="taggroup">
    <h2>
      <xsl:value-of select="@kind"/>
      <span class="badge {@status}"><xsl:value-of select="@status"/></span>
    </h2>
    <p class="name">
      <xsl:value-of select="@names"/>
    </p>
    <xsl:for-each select="text()">
      <xsl:if test="normalize-space(.) != ''">
        <p class="desc"><xsl:value-of select="normalize-space(.)"/></p>
      </xsl:if>
    </xsl:for-each>
    <p class="cite"><xsl:value-of select="@cite"/></p>
  </xsl:template>

</xsl:stylesheet>
