;(function() {
  const header = document.querySelector('header');
  const body = document.querySelector('body');
  const main = document.querySelector('main');
  if(header && body && main){
    let navBtn = header.querySelector('a.nav-btn');
    let navMenu = body.querySelector('nav.nav-menu');
    if(navBtn && navMenu){
      navBtn.addEventListener('click', function(e){
        e.preventDefault();
        navMenu.classList.toggle('active');
      });

      function onResize(){
        if(window.innerWidth >= 1024){
          navMenu.classList.add('active');
        }else{
          navMenu.classList.remove('active');
        }
      }
      onResize();
      window.addEventListener('resize', onResize, {passive: true});
    }
  }

  if(main){
    function loop(){
      main.querySelectorAll('input[type="text"], input[type="number"], input[type="password"], input[type="email"], input[type="tel"], input[type="url"]').forEach(function(elm){
        if(elm.hasAttribute('js-loaded') || elm.parentNode.classList.contains('input-wrapper')){
          return;
        }
        elm.setAttribute('js-loaded', '');

        let label;
        if(elm.id){
          label = elm.parentNode.querySelector(`label[for="${elm.id.replace(/"/g, '')}"]`);
        }
  
        if(!label){
          let name = elm.getAttribute('label');
          if(!name){
            name = elm.getAttribute('placeholder');
          }
          if(!name){
            name = elm.getAttribute('name');
          }
          if(!name){
            return;
          }
  
          if(!elm.id){
            let id = btoa(Math.floor(Math.random() * 10000000000000000).toString()+'-'+Date.now().toString());
            elm.id = 'input-'+id.replace(/=/g, '');
          }
  
          label = document.createElement('label');
          label.textContent = name;
          label.setAttribute('for', elm.id);
        }

        elm.removeAttribute('placeholder');
  
        let wrapper = document.createElement('div');
        wrapper.classList.add('input-wrapper');
        elm.parentNode.insertBefore(wrapper, elm);
        wrapper.appendChild(label);
        wrapper.appendChild(elm);

        elm.addEventListener('input', function(){
          if(elm.value){
            elm.classList.add('has-value');
          }else{
            elm.classList.remove('has-value');
          }
        }, {passive: true});
      });
    }
    loop();
    setInterval(loop, 2500);
  }
})();
