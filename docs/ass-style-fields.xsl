<?xml version="1.0" encoding="UTF-8"?>
<!--
  ass-style-fields.xsl — presentation layer for docs/ass-style-fields.xml.

  Data lives only in the XML; this file holds no behavior claims. Referenced by
  <?xml-stylesheet?> in ass-style-fields.xml. Browsers will not fetch this
  stylesheet over file:// (opaque origins block it), so read the matrix via the
  HTML export. Regenerate after editing the XML, from the repo root:

    $x = New-Object System.Xml.Xsl.XslCompiledTransform
    $x.Load('docs/ass-style-fields.xsl'); $x.Transform('docs/ass-style-fields.xml', 'docs/ass-style-fields.html')

  docs/ass-style-fields.html is gitignored: the XML is the source, the HTML is a view.
-->
<xsl:stylesheet version="1.0"
                xmlns:xsl="http://www.w3.org/1999/XSL/Transform"
                xmlns="http://www.w3.org/1999/xhtml">

  <xsl:output method="html" indent="yes" omit-xml-declaration="yes"/>

  <xsl:template match="/">
    <html>
      <head>
        <title>ASS Style field conversion matrix</title>
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
          .na { color: #999; }
          .note { color: #444; }
          .cite { font-family: ui-monospace, Consolas, monospace; font-size: .78rem; color: #666; }
          .desc { margin: .3rem 0 .6rem; color: #333; }
          .legend { background: #fafafa; border: 1px solid #e5e5e5; padding: .6rem .8rem; font-size: .85rem; }
        </style>
      </head>
      <body>
        <xsl:apply-templates select="matrix"/>
      </body>
    </html>
  </xsl:template>

  <xsl:template match="matrix">
    <h1>ASS Style field conversion matrix</h1>
    <p class="cite">
      <xsl:for-each select="meta/pin">
        <xsl:value-of select="@renderer"/><xsl:text> </xsl:text>
        <code><xsl:value-of select="@commit"/></code>
        <xsl:if test="position() != last()"><xsl:text> · </xsl:text></xsl:if>
      </xsl:for-each>
    </p>

    <div class="legend">
      <strong>How to read a row.</strong> Each field shows the parse step and the
      post step for each pinned renderer. <code>verified</code> names exactly
      which renderer conversion was read at a pinned source line. A conversion
      with no citation is <strong>unmodeled</strong> and is not evidence.
      <span class="badge V">V</span> at least one renderer conversion was read;
      <span class="badge S">S</span> assumed or reported but not verified.
      This matrix records conversions. It does not prove pixel equivalence, and
      it never authorizes a <code>SafeFix</code>.
    </div>

    <h2>Parse vocabulary</h2>
    <table>
      <tr><th>token</th><th>meaning</th></tr>
      <xsl:for-each select="meta/parses/parse">
        <tr>
          <td class="name"><xsl:value-of select="@id"/></td>
          <td><xsl:value-of select="@desc"/></td>
        </tr>
      </xsl:for-each>
    </table>

    <h2>Post vocabulary</h2>
    <table>
      <tr><th>token</th><th>meaning</th></tr>
      <xsl:for-each select="meta/posts/post">
        <tr>
          <td class="name"><xsl:value-of select="@id"/></td>
          <td><xsl:value-of select="@desc"/></td>
        </tr>
      </xsl:for-each>
    </table>

    <h2>Fields</h2>
    <table>
      <tr>
        <th>field</th><th>kind</th><th>renderer</th><th>parse</th><th>post</th>
        <th>verified</th><th>source</th>
      </tr>
      <xsl:for-each select="stylefield">
        <xsl:variable name="rowspan" select="count(conv)"/>
        <xsl:for-each select="conv">
          <tr>
            <xsl:if test="position() = 1">
              <td rowspan="{$rowspan}" class="name">
                <xsl:value-of select="../@name"/>
                <span class="badge {../@status}"><xsl:value-of select="../@status"/></span>
              </td>
              <td rowspan="{$rowspan}"><xsl:value-of select="../@kind"/></td>
            </xsl:if>
            <td class="name"><xsl:value-of select="@renderer"/></td>
            <td><xsl:value-of select="@parse"/></td>
            <td><xsl:value-of select="@post"/></td>
            <td>
              <xsl:choose>
                <xsl:when test="@verified"><xsl:value-of select="@verified"/></xsl:when>
                <xsl:otherwise><span class="na">none</span></xsl:otherwise>
              </xsl:choose>
            </td>
            <td class="cite"><xsl:value-of select="@cite"/></td>
          </tr>
        </xsl:for-each>
      </xsl:for-each>
    </table>

    <h2>Notes and open questions</h2>
    <xsl:for-each select="stylefield[note]">
      <h3><xsl:value-of select="@name"/></h3>
      <p class="desc"><xsl:value-of select="normalize-space(note)"/></p>
    </xsl:for-each>
  </xsl:template>
</xsl:stylesheet>
