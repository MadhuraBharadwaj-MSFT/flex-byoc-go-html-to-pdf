package sample

const InvoiceHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>Sample invoice</title>
  <style>
    @page { size: A4; margin: 0; }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      color: #172033;
      background: #eef2f8;
      font-family: "Noto Sans", "DejaVu Sans", Arial, sans-serif;
      font-size: 13px;
      line-height: 1.45;
    }
    .sheet {
      min-height: 297mm;
      padding: 18mm;
      background: white;
    }
    .accent {
      height: 7px;
      margin: -18mm -18mm 15mm;
      background: linear-gradient(90deg, #635bff, #2d9cdb, #27ae60);
    }
    .header {
      display: flex;
      align-items: flex-start;
      justify-content: space-between;
      gap: 24px;
    }
    .brand {
      display: flex;
      align-items: center;
      gap: 12px;
    }
    .logo {
      display: grid;
      width: 46px;
      height: 46px;
      place-items: center;
      color: white;
      border-radius: 12px;
      background: #635bff;
      font-size: 20px;
      font-weight: 800;
      letter-spacing: -1px;
    }
    .brand h1, .invoice-title h2 { margin: 0; }
    .brand p, .invoice-title p { margin: 3px 0 0; color: #667085; }
    .invoice-title { text-align: right; }
    .invoice-title h2 {
      color: #101828;
      font-size: 30px;
      letter-spacing: 1px;
    }
    .summary {
      display: grid;
      grid-template-columns: 1fr 1fr 1fr;
      gap: 12px;
      margin: 28px 0;
    }
    .card {
      padding: 14px 16px;
      border: 1px solid #e4e7ec;
      border-radius: 10px;
      background: #f9fafb;
    }
    .label {
      margin-bottom: 4px;
      color: #667085;
      font-size: 10px;
      font-weight: 700;
      letter-spacing: .8px;
      text-transform: uppercase;
    }
    .value { color: #101828; font-size: 14px; font-weight: 700; }
    .parties {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 36px;
      margin-bottom: 26px;
    }
    .party {
      padding-top: 12px;
      border-top: 2px solid #eaecf0;
    }
    .party strong { display: block; margin-bottom: 5px; font-size: 15px; }
    .muted { color: #667085; }
    table {
      width: 100%;
      border-collapse: collapse;
      overflow: hidden;
      border: 1px solid #e4e7ec;
      border-radius: 10px;
    }
    th {
      padding: 11px 12px;
      color: #475467;
      background: #f2f4f7;
      font-size: 10px;
      letter-spacing: .65px;
      text-align: left;
      text-transform: uppercase;
    }
    td {
      padding: 13px 12px;
      border-top: 1px solid #eaecf0;
      vertical-align: top;
    }
    th:nth-child(n+2), td:nth-child(n+2) { text-align: right; }
    .item-name { color: #101828; font-weight: 700; }
    .item-note { margin-top: 2px; color: #667085; font-size: 11px; }
    .totals {
      width: 44%;
      margin: 20px 0 0 auto;
    }
    .total-row {
      display: flex;
      justify-content: space-between;
      padding: 7px 0;
      color: #475467;
    }
    .total-row.grand {
      margin-top: 5px;
      padding-top: 12px;
      color: #101828;
      border-top: 2px solid #101828;
      font-size: 18px;
      font-weight: 800;
    }
    .note {
      margin-top: 34px;
      padding: 16px 18px;
      border-left: 4px solid #635bff;
      background: #f4f3ff;
    }
    .footer {
      display: flex;
      justify-content: space-between;
      margin-top: 38px;
      padding-top: 14px;
      color: #667085;
      border-top: 1px solid #eaecf0;
      font-size: 10px;
    }
  </style>
</head>
<body>
  <main class="sheet">
    <div class="accent"></div>
    <header class="header">
      <div class="brand">
        <div class="logo">NW</div>
        <div>
          <h1>Northwind Studio</h1>
          <p>Design systems for growing teams</p>
        </div>
      </div>
      <div class="invoice-title">
        <h2>INVOICE</h2>
        <p>#INV-2026-1108</p>
      </div>
    </header>

    <section class="summary">
      <div class="card">
        <div class="label">Issue date</div>
        <div class="value">November 8, 2026</div>
      </div>
      <div class="card">
        <div class="label">Due date</div>
        <div class="value">December 8, 2026</div>
      </div>
      <div class="card">
        <div class="label">Amount due</div>
        <div class="value">$8,910.00 USD</div>
      </div>
    </section>

    <section class="parties">
      <div class="party">
        <div class="label">From</div>
        <strong>Northwind Studio LLC</strong>
        <div class="muted">100 Market Street<br>Seattle, WA 98101<br>billing@example.com</div>
      </div>
      <div class="party">
        <div class="label">Bill to</div>
        <strong>Contoso Retail</strong>
        <div class="muted">Attn: Accounts Payable<br>200 Commerce Avenue<br>Portland, OR 97205</div>
      </div>
    </section>

    <table>
      <thead>
        <tr>
          <th>Description</th>
          <th>Qty</th>
          <th>Rate</th>
          <th>Amount</th>
        </tr>
      </thead>
      <tbody>
        <tr>
          <td>
            <div class="item-name">Design system foundation</div>
            <div class="item-note">Tokens, accessibility baseline, and component architecture</div>
          </td>
          <td>1</td>
          <td>$4,800.00</td>
          <td>$4,800.00</td>
        </tr>
        <tr>
          <td>
            <div class="item-name">Commerce component library</div>
            <div class="item-note">Product cards, cart patterns, and responsive checkout</div>
          </td>
          <td>32</td>
          <td>$110.00</td>
          <td>$3,520.00</td>
        </tr>
        <tr>
          <td>
            <div class="item-name">Implementation workshop</div>
            <div class="item-note">Remote enablement session and recorded handoff</div>
          </td>
          <td>1</td>
          <td>$600.00</td>
          <td>$600.00</td>
        </tr>
      </tbody>
    </table>

    <section class="totals">
      <div class="total-row"><span>Subtotal</span><strong>$8,920.00</strong></div>
      <div class="total-row"><span>Credit</span><strong>-$10.00</strong></div>
      <div class="total-row grand"><span>Total</span><span>$8,910.00</span></div>
    </section>

    <section class="note">
      <div class="label">Payment note</div>
      Thank you for your business. Please include invoice INV-2026-1108 with your payment.
    </section>

    <footer class="footer">
      <span>This is a fictional invoice generated by the Azure Functions sample.</span>
      <span>northwind.example</span>
    </footer>
  </main>
</body>
</html>`
