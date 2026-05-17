'use strict';

const dot        = document.getElementById('dot');
const statusText = document.getElementById('status-text');
const grid       = document.getElementById('cards-grid');
const emptyState = document.getElementById('empty-state');

const countEls = {
  all:     document.getElementById('count-all'),
  price:   document.getElementById('count-price'),
  float:   document.getElementById('count-float'),
  pattern: document.getElementById('count-pattern'),
};

const counts = { all: 0, price: 0, float: 0, pattern: 0 };
let activeTab = 'price';

// ── Helpers ───────────────────────────────────────────────────────────────────

function cents(c) {
  return '$' + (c / 100).toFixed(2);
}

function ago(isoStr) {
  const secs = Math.floor((Date.now() - new Date(isoStr)) / 1000);
  if (secs < 60)   return `${secs}s ago`;
  if (secs < 3600) return `${Math.floor(secs / 60)}m ago`;
  return `${Math.floor(secs / 3600)}h ago`;
}

function iconSrc(url) {
  if (!url) return '';
  if (url.startsWith('http')) return url;
  return `https://community.cloudflare.steamstatic.com/economy/image/${url}/48fx36f`;
}

function discountClass(d) {
  if (d >= 25) return 'disc-great';
  if (d >= 20) return 'disc-good';
  if (d >= 10) return 'disc-mid';
  if (d > 0)   return 'disc-low';
  return 'disc-neg';
}

function floatClass(f) {
  if (f <= 0.07) return 'float-elite';
  if (f <= 0.12) return 'float-great';
  if (f <= 0.15) return 'float-good';
  return 'float-ok';
}

function liquidityTag(vol) {
  if (!vol || vol === 0) return '<span class="liq-tag liq-none">liq ?</span>';
  if (vol >= 100) return `<span class="liq-tag liq-high" title="${vol} sales/30d">High</span>`;
  if (vol >= 20)  return `<span class="liq-tag liq-medium" title="${vol} sales/30d">Med</span>`;
  return `<span class="liq-tag liq-low" title="${vol} sales/30d">Low</span>`;
}

// ── Card builder ──────────────────────────────────────────────────────────────

function buildCard(deal, isNew) {
  const card = document.createElement('div');
  card.className = 'card' + (isNew ? ' new' : '');
  card.dataset.id       = deal.listing_id;
  card.dataset.strategy = deal.matched_by;

  const disc     = deal.discount > 0 ? deal.discount.toFixed(1) + '%' : '—';
  const discCls  = discountClass(deal.discount);
  const floatStr = deal.float > 0 ? deal.float.toFixed(7) : '—';
  const floatCls = deal.float > 0 ? floatClass(deal.float) : 'float-ok';

  // Stats columns differ by strategy — most important metric goes first
  let stats = '';
  if (deal.matched_by === 'price') {
    stats = `
      <div class="stat">
        <div class="stat-label">Price</div>
        <div class="stat-value">${cents(deal.price)}</div>
      </div>
      <div class="stat">
        <div class="stat-label">Discount</div>
        <div class="stat-value ${discCls}">${disc}</div>
      </div>
      <div class="stat">
        <div class="stat-label">Float</div>
        <div class="stat-value sm ${floatCls}">${floatStr}</div>
      </div>`;
  } else if (deal.matched_by === 'float') {
    stats = `
      <div class="stat">
        <div class="stat-label">Float</div>
        <div class="stat-value sm ${floatCls}">${floatStr}</div>
      </div>
      <div class="stat">
        <div class="stat-label">Price</div>
        <div class="stat-value">${cents(deal.price)}</div>
      </div>
      <div class="stat">
        <div class="stat-label">Discount</div>
        <div class="stat-value ${discCls}">${disc}</div>
      </div>`;
  } else {
    const seed = deal.paint_seed > 0 ? deal.paint_seed : '—';
    stats = `
      <div class="stat">
        <div class="stat-label">Seed</div>
        <div class="stat-value">${seed}</div>
      </div>
      <div class="stat">
        <div class="stat-label">Float</div>
        <div class="stat-value sm ${floatCls}">${floatStr}</div>
      </div>
      <div class="stat">
        <div class="stat-label">Price</div>
        <div class="stat-value">${cents(deal.price)}</div>
      </div>`;
  }

  // Ref prices + savings
  let refs = '';
  const b = deal.buff_price   > 0 ? `<span class="ref-buff"><span class="ref-lbl">B</span>${cents(deal.buff_price)}</span>` : '';
  const y = deal.youpin_price > 0 ? `<span class="ref-youpin"><span class="ref-lbl">Y</span>${cents(deal.youpin_price)}</span>` : '';
  const refVal = deal.ref_price > 0 ? deal.ref_price : 0;
  const saving = refVal > deal.price ? `<span class="ref-save">Save ${cents(refVal - deal.price)}</span>` : '';
  if (b || y) {
    refs = `<div class="card-refs">${[b, y].filter(Boolean).join('')}${saving}</div>`;
  } else if (refVal > 0) {
    refs = `<div class="card-refs"><span style="color:#374151">Ref ${cents(refVal)}</span>${saving}</div>`;
  }

  // Sticker SP tag
  const spTag = deal.sticker_sp > 5
    ? `<span class="sp-tag">SP ${deal.sticker_sp.toFixed(1)}%</span>`
    : '';

  const liqTag = liquidityTag(deal.volume);

  card.innerHTML = `
    <div class="card-meta">
      <span class="badge badge-${deal.matched_by}">${deal.matched_by.toUpperCase()}</span>
      <span class="card-age age" data-ts="${deal.detected_at}">${ago(deal.detected_at)}</span>
    </div>
    <div class="card-item">
      <img class="card-img" src="${iconSrc(deal.icon_url)}" alt="" loading="lazy" onerror="this.style.display='none'"/>
      <div class="card-info">
        <a class="card-name" href="${deal.listing_url}" target="_blank" rel="noopener">${deal.item_name}</a>
        <div class="card-cond">${deal.condition}${spTag}${liqTag}</div>
      </div>
    </div>
    <div class="card-stats">${stats}</div>
    ${refs}
    <div class="card-footer">
      <button class="btn-buy" data-id="${deal.listing_id}">Buy Now</button>
    </div>`;

  card.querySelector('.btn-buy').addEventListener('click', onBuy);
  return card;
}

// ── Insert deal ───────────────────────────────────────────────────────────────

function addDeal(deal, isNew) {
  const card    = buildCard(deal, isNew);
  const visible = activeTab === 'all' || deal.matched_by === activeTab;
  if (!visible) card.style.display = 'none';

  grid.insertBefore(card, emptyState);

  counts.all++;
  counts[deal.matched_by] = (counts[deal.matched_by] || 0) + 1;
  countEls.all.textContent             = counts.all;
  countEls[deal.matched_by].textContent = counts[deal.matched_by];

  if (visible) emptyState.style.display = 'none';
}

// ── Tabs ──────────────────────────────────────────────────────────────────────

const emptyMessages = {
  all:     ['No deals yet',         'Waiting for listings matching your rules…'],
  price:   ['No price deals yet',   'Waiting for listings above the discount threshold…'],
  float:   ['No float deals yet',   'Waiting for items matching float rules…'],
  pattern: ['No pattern deals yet', 'Waiting for items matching the seed list…'],
};

document.querySelectorAll('.tab').forEach(tab => {
  tab.addEventListener('click', () => {
    activeTab = tab.dataset.tab;
    document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
    tab.classList.add('active');

    let anyVisible = false;
    for (const card of grid.querySelectorAll('.card')) {
      const show = activeTab === 'all' || card.dataset.strategy === activeTab;
      card.style.display = show ? '' : 'none';
      if (show) anyVisible = true;
    }

    const [title, sub] = emptyMessages[activeTab];
    emptyState.querySelector('strong').textContent = title;
    emptyState.querySelector('span').textContent   = sub;
    emptyState.style.display = anyVisible ? 'none' : '';
  });
});

// ── Buy handler ───────────────────────────────────────────────────────────────

async function onBuy(e) {
  const btn = e.currentTarget;
  const id  = btn.dataset.id;
  btn.disabled    = true;
  btn.textContent = '…';

  try {
    const res = await fetch(`/buy/${id}`, { method: 'POST' });
    if (res.ok) {
      btn.textContent = 'Bought!';
      btn.classList.add('success');
    } else {
      btn.textContent = 'Failed';
      btn.classList.add('error');
      setTimeout(() => { btn.disabled = false; btn.textContent = 'Buy Now'; btn.classList.remove('error'); }, 3000);
    }
  } catch {
    btn.textContent = 'Error';
    btn.classList.add('error');
    setTimeout(() => { btn.disabled = false; btn.textContent = 'Buy Now'; btn.classList.remove('error'); }, 3000);
  }
}

// ── Age refresh ───────────────────────────────────────────────────────────────

setInterval(() => {
  for (const el of document.querySelectorAll('.age[data-ts]')) {
    el.textContent = ago(el.dataset.ts);
  }
}, 10_000);

// ── Initial load ──────────────────────────────────────────────────────────────

fetch('/deals')
  .then(r => r.json())
  .then(deals => {
    if (!Array.isArray(deals)) return;
    for (const deal of [...deals].reverse()) addDeal(deal, false);
  })
  .catch(err => console.error('failed to load deals:', err));

// ── SSE ───────────────────────────────────────────────────────────────────────

function connectSSE() {
  const es = new EventSource('/events');

  es.onopen = () => {
    dot.classList.add('live');
    statusText.textContent = 'live';
  };

  es.onmessage = (e) => {
    try { addDeal(JSON.parse(e.data), true); }
    catch (err) { console.error('SSE parse error', err); }
  };

  es.onerror = () => {
    dot.classList.remove('live');
    statusText.textContent = 'reconnecting…';
    es.close();
    setTimeout(connectSSE, 3000);
  };
}

connectSSE();
