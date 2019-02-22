describe('#draw', function() {
  var svg, resources, jobs, newUrl;

  beforeEach(function() {
    var svgNode = document.createElement('svg');
    svgNode.innerHTML = "<svg class=\"pipeline-graph\"></svg>"
    document.body.appendChild(svgNode);
    var foundSvg = d3.select('.pipeline-graph');
    svg = createPipelineSvg(foundSvg);
    resources = [
      {
        name: "resource",
        pipeline_name: "pipeline",
        team_name: "main",
        type: "mock",
        last_checked: 1550247797,
        pinned_version: {
          version: "version"
        }
      }
    ];
    jobs = [
        {
          id: 1,
          name: "job",
          pipeline_name: "pipeline",
          team_name: "main",
          next_build: null,
          finished_build: null,
          inputs: [
            {
            name: "resource",
            resource: "resource",
            trigger: false
            }
          ],
          outputs: [],
          groups: []
        }
      ];
    newUrl = { send: function() {} };
  });

  it('resource box grows and shrinks with hover state', function() {
    draw(svg, jobs, resources, newUrl);
    var resourceRect = document.querySelector('.input rect');
    var initialWidth = resourceRect.getBBox().width;
    var initialHeight = resourceRect.getBBox().height;

    resourceRect.dispatchEvent(new MouseEvent('mouseover', {bubbles: true}));

    expect(Math.floor(resourceRect.getBBox().width))
      .toBe(Math.floor(initialWidth * 1.09));
    expect(Math.floor(resourceRect.getBBox().height))
      .toBe(Math.floor(initialHeight * 1.09));

    resourceRect.dispatchEvent(new MouseEvent('mouseout', {bubbles: true}));

    expect(resourceRect.getBBox().width).toBe(initialWidth);
    expect(resourceRect.getBBox().height).toBe(initialHeight);
  });

  it('resource text font size changes with hover state', function() {
    draw(svg, jobs, resources, newUrl);
    var resourceText = document.querySelector('.input text');
    var initialFontSize = window.getComputedStyle(resourceText).fontSize;

    resourceText.dispatchEvent(new MouseEvent('mouseover', {bubbles: true}));

    console.log(document.body);

    expect(window.getComputedStyle(resourceText).fontSize)
      .not.toBe(initialFontSize);

    resourceText.dispatchEvent(new MouseEvent('mouseout', {bubbles: true}));

    expect(window.getComputedStyle(resourceText).fontSize)
      .toBe(initialFontSize);
  });

  it('resource repositions with hover state', function() {
    draw(svg, jobs, resources, newUrl);
    var resourceRect = document.querySelector('.input rect');
    var initialWidth = resourceRect.getBoundingClientRect().width;
    var initialX = resourceRect.getBoundingClientRect().x;
    var initialHeight = resourceRect.getBoundingClientRect().height;
    var initialY = resourceRect.getBoundingClientRect().y;

    resourceRect.dispatchEvent(new MouseEvent('mouseover', {bubbles: true}));

    var newWidth = initialWidth * 1.09;
    var deltaX = 0.045 * initialWidth;
    var newHeight = initialHeight * 1.09;
    var deltaY = 0.045 * initialHeight;

    expect(Math.floor(resourceRect.getBoundingClientRect().x))
      .toBe(Math.floor(initialX - deltaX));
    expect(Math.floor(resourceRect.getBoundingClientRect().y))
      .toBe(Math.floor(initialY - deltaY));

    resourceRect.dispatchEvent(new MouseEvent('mouseout', {bubbles: true}));

    expect(resourceRect.getBoundingClientRect().x).toBe(initialX);
    expect(resourceRect.getBoundingClientRect().y).toBe(initialY);
  });

  it('resource text repositions with hover state', function() {
    draw(svg, jobs, resources, newUrl);
    var resourceText = document.querySelector('.input text');
    var resourceRect = document.querySelector('.input rect');

    var initialTextRight = resourceText.getBoundingClientRect().right;
    var initialRectRight = resourceRect.getBoundingClientRect().right;

    resourceText.dispatchEvent(new MouseEvent('mouseover', {bubbles: true}));

    var textRight = resourceText.getBoundingClientRect().right;
    var rectRight = resourceRect.getBoundingClientRect().right;

    spacingRatio = (rectRight - textRight) /
      (initialRectRight - initialTextRight);

    expect(Math.round(spacingRatio * 100)).toBe(109);

    resourceText.dispatchEvent(new MouseEvent('mouseout', {bubbles: true}));

    expect(resourceText.getBoundingClientRect().right).toBe(initialTextRight);
  });

  describe('when job has input with trigger: false', function() {
    beforeEach(function() {
      jobs = [
        {
          id: 1,
          name: "job",
          pipeline_name: "pipeline",
          team_name: "main",
          next_build: null,
          finished_build: null,
          inputs: [
            {
            name: "resource",
            resource: "resource",
            trigger: false
            }
          ],
          outputs: [],
          groups: []
        }
      ];
    });

    it('adds trigger-false class to edge', function() {
      draw(svg, jobs, resources, newUrl);
      var edge = document.querySelector('.edge');
      expect(edge.classList.contains('trigger-false')).toBe(true);
    });
  });

  describe('when job has input with trigger: true', function() {
    beforeEach(function() {
      jobs = [
        {
          id: 1,
          name: "job",
          pipeline_name: "pipeline",
          team_name: "main",
          next_build: null,
          finished_build: null,
          inputs: [
            {
            name: "resource",
            resource: "resource",
            trigger: true
            }
          ],
          outputs: [],
          groups: []
        }
      ];
    });

    it('does not add trigger-false class to edge', function() {
      draw(svg, jobs, resources, newUrl);
      var edge = document.querySelector('.edge');
      expect(edge.classList.contains('trigger-false')).toBe(false);
    });
  });
});
