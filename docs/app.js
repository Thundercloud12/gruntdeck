document.addEventListener('DOMContentLoaded', () => {
  setupQuickstartTabs();
  setupSidebarNavigation();
});

// Quickstart Code Carousel Tabs
function setupQuickstartTabs() {
  const tabs = document.querySelectorAll('.qs-tab');
  const contents = document.querySelectorAll('.qs-content');

  tabs.forEach(tab => {
    tab.addEventListener('click', () => {
      tabs.forEach(t => t.classList.remove('active'));
      contents.forEach(c => c.classList.remove('active'));

      tab.classList.add('active');
      const targetId = tab.getAttribute('data-target');
      const targetContent = document.getElementById(targetId);
      if (targetContent) {
        targetContent.classList.add('active');
      }
    });
  });
}

// Docs Sidebar Active Link Highlight on Scroll
function setupSidebarNavigation() {
  const links = document.querySelectorAll('.docs-menu-link');
  const sections = document.querySelectorAll('.docs-content section');

  if (sections.length === 0) return;

  window.addEventListener('scroll', () => {
    let current = '';
    sections.forEach(section => {
      const sectionTop = section.offsetTop;
      if (pageYOffset >= sectionTop - 120) {
        current = section.getAttribute('id');
      }
    });

    links.forEach(link => {
      link.classList.remove('active');
      if (link.getAttribute('href') === `#${current}`) {
        link.classList.add('active');
      }
    });
  });
}

// Copy Code Snippet
function copyCode(buttonElement, targetId) {
  const codeElem = document.getElementById(targetId);
  if (!codeElem) return;

  const textToCopy = codeElem.textContent.trim();
  navigator.clipboard.writeText(textToCopy).then(() => {
    const originalText = buttonElement.textContent;
    buttonElement.textContent = '✓ Copied!';
    buttonElement.style.color = '#22c55e';
    setTimeout(() => {
      buttonElement.textContent = originalText;
      buttonElement.style.color = '';
    }, 2000);
  }).catch(err => {
    console.error('Failed to copy text', err);
  });
}
