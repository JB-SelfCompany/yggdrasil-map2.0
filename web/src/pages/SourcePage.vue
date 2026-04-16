<template>
  <div class="source-page">
    <div class="card">
      <h2>Source Code</h2>
      <p>yggmap is an open-source network topology visualizer for the Yggdrasil mesh network.</p>
      <a
        href="https://github.com/JB-SelfCompany/yggdrasil-map2.0"
        target="_blank"
        rel="noopener noreferrer"
        class="gh-link"
      >
        View on GitHub &rarr;
      </a>

      <h2>Tech Stack</h2>
      <table class="info-table">
        <tr>
          <th>Backend</th>
          <td>Go 1.21+ &mdash; stdlib net/http, gorilla/websocket</td>
        </tr>
        <tr>
          <th>Frontend</th>
          <td>Vue 3 + TypeScript + Vite</td>
        </tr>
        <tr>
          <th>Graph</th>
          <td>Sigma.js v3 + graphology + ForceAtlas2</td>
        </tr>
        <tr>
          <th>State</th>
          <td>Pinia + VueUse</td>
        </tr>
        <tr>
          <th>Protocol</th>
          <td>Yggdrasil admin socket JSON-RPC</td>
        </tr>
      </table>

      <h2>License</h2>
      <p>Released under the GPLv3 License.</p>
    </div>
  </div>
</template>

<style scoped>
.source-page {
  width: 100%;
  height: 100%;
  overflow-y: auto;
  background: #F5F5F5;
  padding: 24px 16px;
  box-sizing: border-box;
}

.card {
  max-width: 640px;
  margin: 0 auto;
  background: #FFFFFF;
  border: 1px solid #E5E7EB;
  border-radius: 8px;
  padding: 24px 28px;
}

h2 {
  margin: 0 0 12px;
  font-family: 'Ubuntu Mono', 'Consolas', monospace;
  font-size: 15px;
  font-weight: 600;
  color: #29BBFF;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

h2 + p,
h2 + table,
h2 + a {
  margin-top: 0;
}

p + h2,
table + h2,
a + h2 {
  margin-top: 28px;
}

p {
  margin: 0 0 16px;
  font-size: 13px;
  color: #333333;
  line-height: 1.6;
}

.gh-link {
  display: inline-block;
  margin-bottom: 0;
  padding: 8px 16px;
  background: #F5F5F5;
  border: 1px solid #E5E7EB;
  border-radius: 6px;
  font-family: 'Ubuntu Mono', 'Consolas', monospace;
  font-size: 13px;
  color: #29BBFF;
  text-decoration: none;
  transition: background 0.15s, border-color 0.15s;
}

.gh-link:hover {
  background: #E5E7EB;
  border-color: #29BBFF;
}

.info-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
  color: #333333;
  margin-bottom: 0;
}

.info-table tr {
  border-bottom: 1px solid #E5E7EB;
}

.info-table tr:last-child {
  border-bottom: none;
}

.info-table th {
  width: 30%;
  padding: 8px 4px 8px 0;
  font-weight: 500;
  color: #888888;
  text-align: left;
  white-space: nowrap;
  font-family: 'Ubuntu Mono', 'Consolas', monospace;
}

.info-table td {
  padding: 8px 4px;
  color: #333333;
}
</style>
