package com.bigBrother.api.filters;

import com.bigBrother.api.services.JwtService;
import com.bigBrother.api.repositories.UserRepository;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.security.authentication.UsernamePasswordAuthenticationToken;
import org.springframework.security.core.context.SecurityContextHolder;
import org.springframework.security.core.userdetails.UserDetails;
import org.springframework.security.web.authentication.WebAuthenticationDetailsSource;
import org.springframework.stereotype.Component;
import org.springframework.web.filter.OncePerRequestFilter;

import jakarta.servlet.FilterChain;
import jakarta.servlet.ServletException;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import java.io.IOException;

@Component
public class JwtAuthenticationFilter extends OncePerRequestFilter {

    @Autowired
    private JwtService jwtService;

    @Autowired
    private UserRepository userRepository;

    @Value("${internal.api.token}")
    private String internalApiToken;

    @Override
    protected void doFilterInternal(HttpServletRequest request, HttpServletResponse response, FilterChain chain)
            throws ServletException, IOException {
        final String authHeader = request.getHeader("Authorization");
        final String jwt;
        final String username;

        // Permitir autenticación por token interno para /api/cameras
        String path = request.getRequestURI();
        if (path.startsWith("/api/cameras")) {
            if (authHeader != null && authHeader.equals("Bearer " + internalApiToken)) {
                // Autenticación interna: crea un token de autenticación simple
                UsernamePasswordAuthenticationToken authToken =
                        new UsernamePasswordAuthenticationToken("internal-service", null, java.util.Collections.emptyList());
                authToken.setDetails(new WebAuthenticationDetailsSource().buildDetails(request));
                SecurityContextHolder.getContext().setAuthentication(authToken);
                chain.doFilter(request, response);
                return;
            }
        }

        // Verifica si el encabezado contiene el token JWT
        if (authHeader == null || !authHeader.startsWith("Bearer ")) {
            chain.doFilter(request, response);
            return;
        }

        jwt = authHeader.substring(7); // Extrae el token después de "Bearer "
        username = jwtService.extractUsername(jwt); // Extrae el username del token

        // Si el usuario no está autenticado, realiza la autenticación
        if (username != null && SecurityContextHolder.getContext().getAuthentication() == null) {
            UserDetails userDetails = userRepository.findByUsername(username)
                    .orElse(null);

            if (userDetails != null && jwtService.validateToken(jwt, userDetails)) {
                var authToken = new UsernamePasswordAuthenticationToken(
                        userDetails, null, userDetails.getAuthorities());
                authToken.setDetails(new WebAuthenticationDetailsSource().buildDetails(request));
                SecurityContextHolder.getContext().setAuthentication(authToken);
            }
        }

        chain.doFilter(request, response);
    }
}