var resizeTimer;

app.ports.pinTeamNames.subscribe(function(config) {
  function sections() {
    return Array.from(document.querySelectorAll("." + config.sectionClass));
  }

  function hasClass(node, cls) {
    return node.classList && node.classList.contains(cls);
  }

  function header(section) {
    return Array.from(section.childNodes)
      .find(n => hasClass(n, config.sectionHeaderClass));
  }

  function body(section) {
    return Array.from(section.childNodes)
      .find(n => hasClass(n, config.sectionBodyClass));
  }

  function lowestHeaderTop(section) {
    return body(section).offsetTop
      + body(section).scrollHeight
      - header(section).scrollHeight;
  }

  pageHeaderHeight = () => config.pageHeaderHeight;
  viewportTop = () => window.pageYOffset + pageHeaderHeight();

  updateHeader = (section) => {
    var scrolledFarEnough = section.offsetTop < viewportTop();
    var scrolledTooFar = lowestHeaderTop(section) < viewportTop();
    if (!scrolledFarEnough && !scrolledTooFar) {
      header(section).style.top = "";
      header(section).style.position = "";
      body(section).style.paddingTop = "";
      return 'static';
    } else if (scrolledFarEnough && !scrolledTooFar) {
      header(section).style.position = 'fixed';
      header(section).style.top = pageHeaderHeight() + "px";
      body(section).style.paddingTop = header(section).scrollHeight + "px";
      return 'fixed';
    } else if (scrolledFarEnough && scrolledTooFar) {
      header(section).style.position = 'absolute';
      header(section).style.top = lowestHeaderTop(section) + "px";
      return 'absolute';
    } else if (!scrolledFarEnough && scrolledTooFar) {
      return 'impossible';
    }
  }

  updateSticky = () => {
    document.querySelector("." + config.pageBodyClass).style.marginTop = pageHeaderHeight();
    sections().forEach(updateHeader);
  }

  clearTimeout(resizeTimer);
  resizeTimer = setTimeout(updateSticky, 250);
  window.onscroll = updateSticky;
});

app.ports.resetPipelineFocus.subscribe(resetPipelineFocus);

app.ports.renderPipeline.subscribe(function (values) {
  setTimeout(function(){ // elm 0.17 bug, see https://github.com/elm-lang/core/issues/595
    foundSvg = d3.select(".pipeline-graph");
    var svg = createPipelineSvg(foundSvg)
    if (svg.node() != null) {
      var jobs = values[0];
      var resources = values[1];
      draw(svg, jobs, resources, app.ports.newUrl);
    }
  }, 0)
});

app.ports.requestLoginRedirect.subscribe(function (message) {
  var path = document.location.pathname;
  var query = document.location.search;
  var redirect = encodeURIComponent(path + query);
  var loginUrl = "/login?redirect_uri="+ redirect;
  document.location.href = loginUrl;
});

app.ports.setTitle.subscribe(function(title) {
  setTimeout(function(){
    document.title = title + "Concourse";
  }, 0)
});

app.ports.tooltip.subscribe(function (pipelineInfo) {
  var pipelineName = pipelineInfo[0];
  var pipelineTeamName = pipelineInfo[1];

  var team = $('div[id="' + pipelineTeamName + '"]');
  var title = team.find('.card[data-pipeline-name="' + pipelineName + '"]').find('.dashboard-pipeline-name');

  if(title.get(0).offsetWidth < title.get(0).scrollWidth){
    title.parent().attr('data-tooltip', pipelineName);
  }
});

app.ports.tooltipHd.subscribe(function (pipelineInfo) {
  var pipelineName = pipelineInfo[0];
  var pipelineTeamName = pipelineInfo[1];

  var title = $('.card[data-pipeline-name="' + pipelineName + '"][data-team-name="' + pipelineTeamName + '"]').find('.dashboardhd-pipeline-name');

  if(title.get(0).offsetWidth < title.get(0).scrollWidth){
    title.parent().attr('data-tooltip', pipelineName);
  }
});

var storageKey = "csrf_token";
app.ports.saveToken.subscribe(function(value) {
  localStorage.setItem(storageKey, value);
});
app.ports.loadToken.subscribe(function() {
  app.ports.tokenReceived.send(localStorage.getItem(storageKey));
});
window.addEventListener('storage', function(event) {
  if (event.key == storageKey) {
    app.ports.tokenReceived.send(localStorage.getItem(storageKey));
  }
}, false);

var clipboard = new ClipboardJS('#copy-token');
